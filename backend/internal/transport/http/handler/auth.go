package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/config"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/auth"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
	mw "github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/middleware"
)

// refreshCookieName — имя httpOnly-cookie c refresh-токеном. Менять
// можно безболезненно, но клиент об этом имени не знает (только
// браузер) — он работает с куками по протоколу cookie-jar.
const refreshCookieName = "refresh_token"

// AuthHandler собирает зависимости HTTP-слоя авторизации.
type AuthHandler struct {
	auth *auth.Service
	cfg  *config.Config
}

// NewAuthHandler возвращает обработчики с настроенными зависимостями.
func NewAuthHandler(svc *auth.Service, cfg *config.Config) *AuthHandler {
	return &AuthHandler{auth: svc, cfg: cfg}
}

// Register — POST /api/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON или повреждено")
		return
	}

	res, err := h.auth.Register(r.Context(), auth.RegisterInput{
		Email:      req.Email,
		Password:   req.Password,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		MiddleName: req.MiddleName,
		Birthday:   req.Birthday,
		GroupName:  req.GroupName,
	}, clientIP(r))
	if err != nil {
		mapAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, res.Pair.RefreshToken, res.Pair.RefreshExpires)
	writeJSON(w, http.StatusCreated, dto.AuthResponse{
		AccessToken:     res.Pair.AccessToken,
		AccessExpiresAt: res.Pair.AccessExpires,
		TokenType:       "Bearer",
		User:            userPtr(dto.FromUser(res.User)),
	})
}

// Login — POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "тело запроса не JSON")
		return
	}

	res, err := h.auth.Login(r.Context(), req.Email, req.Password, clientIP(r))
	if err != nil {
		mapAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, res.Pair.RefreshToken, res.Pair.RefreshExpires)
	writeJSON(w, http.StatusOK, dto.AuthResponse{
		AccessToken:     res.Pair.AccessToken,
		AccessExpiresAt: res.Pair.AccessExpires,
		TokenType:       "Bearer",
		User:            userPtr(dto.FromUser(res.User)),
	})
}

// Refresh — POST /api/auth/refresh.
// Читает refresh-токен из cookie, обменивает на новую пару и
// возвращает новый access. Старый refresh помечается used в БД
// (one-use), новый ставится в cookie тем же именем.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "refresh_missing", "refresh cookie отсутствует")
		return
	}

	pair, err := h.auth.Refresh(r.Context(), cookie.Value, clientIP(r))
	if err != nil {
		// На любую ошибку refresh немедленно сносим cookie с клиента —
		// иначе фронт будет бесконечно ретраить с тем же значением.
		h.clearRefreshCookie(w)
		mapAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken, pair.RefreshExpires)
	writeJSON(w, http.StatusOK, dto.AuthResponse{
		AccessToken:     pair.AccessToken,
		AccessExpiresAt: pair.AccessExpires,
		TokenType:       "Bearer",
	})
}

// Logout — POST /api/auth/logout. Защищён Auth-middleware: tokenID
// читается из контекста.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	tokenID, ok := mw.TokenID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	if err := h.auth.Logout(r.Context(), tokenID); err != nil {
		mapAuthError(w, err)
		return
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me — GET /api/auth/me. Защищён Auth-middleware.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	profile, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		mapAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.ProfileResponse{
		User:        dto.FromUser(profile.User),
		Roles:       dto.FromRoles(profile.Roles),
		Permissions: profile.Permissions,
	})
}

// setRefreshCookie ставит refresh-токен в httpOnly-cookie с
// параметрами из config (Secure/SameSite/Domain/Path).
func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     h.cfg.CookiePath,
		Domain:   h.cfg.CookieDomain,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: h.cfg.CookieSameSite,
	})
}

// clearRefreshCookie сбрасывает cookie на клиенте (Max-Age=-1).
func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     h.cfg.CookiePath,
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: h.cfg.CookieSameSite,
	})
}

// userPtr — &-хелпер для UserResponse, чтобы код handler'ов оставался однострочным.
func userPtr(u dto.UserResponse) *dto.UserResponse { return &u }

// clientIP пытается определить клиентский IP. Доверяем X-Forwarded-For
// только если поставлен RealIP-middleware (chi.middleware.RealIP),
// который её уже перенёс в r.RemoteAddr.
func clientIP(r *http.Request) string {
	if r.RemoteAddr == "" {
		return ""
	}
	// RemoteAddr формата "host:port" — отрезаем порт.
	for i := len(r.RemoteAddr) - 1; i >= 0; i-- {
		if r.RemoteAddr[i] == ':' {
			return r.RemoteAddr[:i]
		}
	}
	return r.RemoteAddr
}
