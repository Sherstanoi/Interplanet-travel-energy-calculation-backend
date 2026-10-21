// Package auth - создание и проверка JWT.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "interplanet-backend"

// Claims - содержимое (payload) токена: стандартные поля RFC 7519 + наши данные.
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
	// RegisteredClaims.ID - это поле jti: уникальный номер токена.
	// Именно его мы кладем в blacklist Redis при logout.
}

// TokenManager выпускает и проверяет токены.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

// TTL - срок жизни выпускаемых токенов.
func (m *TokenManager) TTL() time.Duration { return m.ttl }

func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Generate подписывает токен алгоритмом HS256.
func (m *TokenManager) Generate(userID int, username string) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", fmt.Errorf("не удалось создать jti: %w", err)
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    issuer,
			Subject:   strconv.Itoa(userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse проверяет подпись, алгоритм и срок действия и возвращает содержимое токена.
func (m *TokenManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // защита от подмены алгоритма (alg=none)
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(issuer),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.ID == "" || claims.UserID <= 0 {
		return nil, errors.New("некорректный токен")
	}
	return claims, nil
}
