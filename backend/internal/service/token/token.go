// Package token реализует выдачу и проверку JWT access-токенов и
// случайных refresh-токенов с хранением их хешей в БД.
//
// Безопасные свойства:
//   - access — короткоживущий JWT (cfg.AccessTokenTTL), подписанный HS256.
//     В БД хранится только SHA-256 хеш, что позволяет отзывать токены до
//     истечения через флаг is_revoked.
//   - refresh — длинная случайная строка, тоже хранится только как хеш.
//     One-use: при выдаче новой пары старый refresh помечается is_used.
//     Если кто-то попробует использовать его второй раз — это признак
//     утечки, сервис ревокует все access-токены пользователя.
//   - jti JWT-claim совпадает с id строки в access_tokens: middleware
//     может найти запись и проверить is_revoked / expires_at, не доверяя
//     только exp из JWT.
package token

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// Issuer — иммутабельный идентификатор приложения в JWT-claims.
// При смене проекта менять здесь и обновлять старые токены не нужно
// (просто перестанут валидироваться).
const Issuer = "academic-debts"

// Sentinel-ошибки, по которым сервисный слой различает причины отказа.
// HTTP-слой мапит их в коды ответа (401/403/etc).
var (
	ErrTokenInvalid     = errors.New("token: подпись или формат некорректны")
	ErrTokenExpired     = errors.New("token: срок действия истёк")
	ErrTokenRevoked     = errors.New("token: отозван или не найден")
	ErrRefreshReplay    = errors.New("token: повторное использование refresh (replay)")
	ErrRefreshNotFound  = errors.New("token: refresh не найден")
	ErrRefreshExhausted = errors.New("token: refresh использован или отозван")
)

// Pair — результат выдачи пары токенов клиенту.
// Plaintext-значения здесь живут только в момент возврата handler'у;
// в БД пишутся только хеши.
type Pair struct {
	AccessToken    string
	AccessExpires  time.Time
	RefreshToken   string
	RefreshExpires time.Time
	AccessTokenID  uuid.UUID // jti, используется для точечной ревокации (logout)
}

// Service инкапсулирует выдачу/проверку токенов. Концептуально stateless,
// зависит только от *repo.Store, секрета и TTL.
type Service struct {
	store      *repo.Store
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time // подменяется в тестах
}

// New собирает Service. accessTTL/refreshTTL обычно приходят из config.
func New(store *repo.Store, secret []byte, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		store:      store,
		secret:     secret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// Claims описывает payload access-JWT.
type Claims struct {
	jwt.RegisteredClaims
}

// Issue создаёт новую пару access+refresh для userID. ip опционален,
// записывается в access_tokens для аудита (история сессий).
func (s *Service) Issue(ctx context.Context, userID uuid.UUID, ip string) (*Pair, error) {
	accessID := uuid.New()
	now := s.now().UTC()
	accessExpires := now.Add(s.accessTTL)
	refreshExpires := now.Add(s.refreshTTL)

	accessJWT, err := s.signAccess(accessID, userID, now, accessExpires)
	if err != nil {
		return nil, fmt.Errorf("sign access: %w", err)
	}

	refreshPlain, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh: %w", err)
	}

	accessHash := hashToken(accessJWT)
	refreshHash := hashToken(refreshPlain)

	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		_, err := q.CreateAccessToken(ctx, queries.CreateAccessTokenParams{
			ID:        pgutil.PgUUID(accessID),
			UserID:    pgutil.PgUUID(userID),
			TokenHash: accessHash,
			ExpiresAt: pgtype.Timestamptz{Time: accessExpires, Valid: true},
			IpAddress: stringPtr(ip),
		})
		if err != nil {
			return fmt.Errorf("insert access: %w", err)
		}
		_, err = q.CreateRefreshToken(ctx, queries.CreateRefreshTokenParams{
			AccessTokenID: pgutil.PgUUID(accessID),
			TokenHash:     refreshHash,
			ExpiresAt:     pgtype.Timestamptz{Time: refreshExpires, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("insert refresh: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Pair{
		AccessToken:    accessJWT,
		AccessExpires:  accessExpires,
		RefreshToken:   refreshPlain,
		RefreshExpires: refreshExpires,
		AccessTokenID:  accessID,
	}, nil
}

// Validate проверяет access-JWT: подпись, срок и существование
// активной записи в access_tokens. При успехе возвращает userID и
// tokenID (он же jti) — последний полезен для logout-точно-этой-сессии.
func (s *Service) Validate(ctx context.Context, accessJWT string) (userID, tokenID uuid.UUID, err error) {
	claims, err := s.parseAccess(accessJWT)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	tokenID, err = uuid.Parse(claims.ID)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrTokenInvalid
	}
	userID, err = uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrTokenInvalid
	}

	row, err := s.store.GetAccessTokenByHash(ctx, hashToken(accessJWT))
	if err != nil {
		if repo.IsNotFound(err) {
			return uuid.Nil, uuid.Nil, ErrTokenRevoked
		}
		return uuid.Nil, uuid.Nil, fmt.Errorf("lookup access: %w", err)
	}

	// Touch last_used_at — best effort, ошибка не критична для запроса.
	_ = s.store.TouchAccessToken(ctx, row.ID)

	return userID, tokenID, nil
}

// Rotate обменивает refresh-токен на новую пару. При обнаружении
// повторного использования (is_used=TRUE) отзывает все access-токены
// пользователя — это защита от кражи refresh.
func (s *Service) Rotate(ctx context.Context, refreshPlain, ip string) (*Pair, error) {
	row, err := s.store.GetRefreshTokenByHash(ctx, hashToken(refreshPlain))
	if err != nil {
		if repo.IsNotFound(err) {
			return nil, ErrRefreshNotFound
		}
		return nil, fmt.Errorf("lookup refresh: %w", err)
	}

	now := s.now().UTC()
	if row.ExpiresAt.Time.Before(now) {
		return nil, ErrRefreshExhausted
	}
	if row.IsRevoked {
		return nil, ErrRefreshExhausted
	}
	if row.IsUsed {
		// Replay: ревокуем все access-токены пользователя, чтобы
		// разорвать сессию атакующего. Узнаём userID через
		// access_token, к которому привязан этот refresh.
		access, lookupErr := s.store.GetAccessTokenByID(ctx, row.AccessTokenID)
		if lookupErr == nil {
			_ = s.store.RevokeAllAccessTokensForUser(ctx, access.UserID)
		}
		return nil, ErrRefreshReplay
	}

	access, err := s.store.GetAccessTokenByID(ctx, row.AccessTokenID)
	if err != nil {
		return nil, fmt.Errorf("lookup access for refresh: %w", err)
	}

	// Помечаем старый refresh использованным и ревокуем старый access
	// в одной транзакции — это атомарный rollover.
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		if err := q.MarkRefreshTokenUsed(ctx, row.ID); err != nil {
			return err
		}
		return q.RevokeAccessToken(ctx, access.ID)
	})
	if err != nil {
		return nil, fmt.Errorf("rotate tx: %w", err)
	}

	return s.Issue(ctx, pgutil.UUID(access.UserID), ip)
}

// Revoke отзывает конкретный access-токен (logout текущей сессии).
func (s *Service) Revoke(ctx context.Context, accessTokenID uuid.UUID) error {
	return s.store.RevokeAccessToken(ctx, pgutil.PgUUID(accessTokenID))
}

// RevokeAll отзывает все access-токены пользователя (logout-all).
func (s *Service) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	return s.store.RevokeAllAccessTokensForUser(ctx, pgutil.PgUUID(userID))
}

func (s *Service) signAccess(accessID, userID uuid.UUID, issuedAt, expiresAt time.Time) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   userID.String(),
			ID:        accessID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

func (s *Service) parseAccess(accessJWT string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(accessJWT, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return s.secret, nil
	}, jwt.WithIssuer(Issuer), jwt.WithExpirationRequired(), jwt.WithLeeway(5*time.Second))
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, ErrTokenExpired
		case errors.Is(err, jwt.ErrTokenNotValidYet),
			errors.Is(err, jwt.ErrTokenSignatureInvalid),
			errors.Is(err, jwt.ErrTokenInvalidIssuer),
			errors.Is(err, jwt.ErrTokenMalformed):
			return nil, ErrTokenInvalid
		default:
			return nil, ErrTokenInvalid
		}
	}
	return claims, nil
}

// generateRefreshToken создаёт криптографически стойкий refresh-токен
// в виде 32 случайных байт, закодированных в hex (64 символа).
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
