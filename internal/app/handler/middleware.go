package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"lab1/internal/app/auth"
)

const bearerPrefix = "Bearer "

// Ключи, под которыми middleware кладет данные пользователя в gin.Context.
const (
	ctxUserID = "user_id"
	ctxClaims = "claims"
)

// currentUserID возвращает id пользователя из токена или 0, если запрос от гостя.
// Создатель услуги берется только отсюда, а не из тела запроса.
func currentUserID(ctx *gin.Context) int {
	return ctx.GetInt(ctxUserID)
}

// authenticate проверяет заголовок Authorization: Bearer <jwt>.
// Возвращает данные токена либо HTTP-код и причину отказа.
func (h *Handler) authenticate(ctx *gin.Context) (*auth.Claims, int, error) {
	header := ctx.GetHeader("Authorization")
	if header == "" {
		return nil, http.StatusUnauthorized, errors.New("требуется авторизация: нет заголовка Authorization")
	}
	if !strings.HasPrefix(header, bearerPrefix) {
		return nil, http.StatusUnauthorized, errors.New("ожидается заголовок «Authorization: Bearer <токен>»")
	}

	// 1. подпись, алгоритм и срок действия
	claims, err := h.Tokens.Parse(strings.TrimSpace(header[len(bearerPrefix):]))
	if err != nil {
		return nil, http.StatusUnauthorized, errors.New("токен недействителен или просрочен")
	}

	// 2. blacklist: токен мог быть отозван через logout до окончания срока действия
	revoked, err := h.Redis.IsJWTRevoked(ctx.Request.Context(), claims.ID)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("не удалось проверить blacklist токенов")
	}
	if revoked {
		return nil, http.StatusUnauthorized, errors.New("токен отозван (выполнен logout)")
	}
	return claims, 0, nil
}

func setUser(ctx *gin.Context, claims *auth.Claims) {
	ctx.Set(ctxUserID, claims.UserID)
	ctx.Set(ctxClaims, claims)
}

// AuthRequired - для методов создателя: без действующего токена 401.
func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		claims, code, err := h.authenticate(ctx)
		if err != nil {
			if code == http.StatusUnauthorized {
				ctx.Header("WWW-Authenticate", "Bearer")
			}
			h.errorHandler(ctx, code, err)
			ctx.Abort()
			return
		}
		setUser(ctx, claims)
		ctx.Next()
	}
}

// AuthOptional - для публичного чтения: токен учитывается, если он есть и верный,
// а во всех остальных случаях запрос обрабатывается как запрос гостя.
func (h *Handler) AuthOptional() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.GetHeader("Authorization") != "" {
			claims, _, err := h.authenticate(ctx)
			if err != nil {
				logrus.Warn("публичный метод: токен проигнорирован: ", err)
			} else {
				setUser(ctx, claims)
			}
		}
		ctx.Next()
	}
}
