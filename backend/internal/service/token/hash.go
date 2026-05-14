package token

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashToken возвращает SHA-256 хеш токена в виде 64-символьной hex-строки.
// Соответствует VARCHAR(64) в access_tokens.token_hash и
// refresh_tokens.token_hash. Используется и для access (JWT), и для
// refresh — клиенту отдаём plaintext, в БД храним только хеш.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
