package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"lab1/internal/app/auth"
	"lab1/internal/app/ds"
	"lab1/internal/app/repository"
)

// RegisterRequest - тело регистрации.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,max=50" example:"alice"`
	Password string `json:"password" binding:"required,max=72" example:"secret123"` // 72 байта - предел bcrypt
}

// UserResponse - созданный пользователь (пароль и его хеш наружу не отдаются).
type UserResponse struct {
	ID       int    `json:"id" example:"1"`
	Username string `json:"username" example:"alice"`
}

// LoginRequest - тело аутентификации.
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"alice"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

// LoginResponse - выданный JWT.
type LoginResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType   string `json:"token_type" example:"Bearer"`
	ExpiresIn   int    `json:"expires_in" example:"3600"` // секунд
}

// RegisterUserAPI godoc
// @Summary      Регистрация
// @Description  Создаёт пользователя. Пароль сохраняется только в виде bcrypt-хеша. Токен при регистрации не выдаётся: после неё нужно вызвать /api/auth/login.
// @Tags         Пользователи
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterRequest  true  "Логин и пароль"
// @Success      201   {object}  UserResponse
// @Failure      400   {object}  ErrorResponse  "Некорректные данные"
// @Failure      409   {object}  ErrorResponse  "Логин уже занят"
// @Failure      500   {object}  ErrorResponse
// @Router       /api/users [post]
func (h *Handler) RegisterUserAPI(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	user := ds.User{Username: req.Username, Password: string(hash)}
	if err := h.Repository.CreateUser(&user); err != nil {
		if errors.Is(err, repository.ErrUsernameTaken) {
			h.errorHandler(ctx, http.StatusConflict, err)
			return
		}
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, UserResponse{ID: user.ID, Username: user.Username})
}

// LoginAPI godoc
// @Summary      Аутентификация
// @Description  Проверяет логин и пароль и выдаёт JWT (HS256). Токен передаётся в заголовке «Authorization: Bearer <токен>».
// @Description  В Swagger: нажмите Authorize и вставьте «Bearer <access_token>».
// @Tags         Пользователи
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "Логин и пароль"
// @Success      200   {object}  LoginResponse
// @Failure      400   {object}  ErrorResponse  "Некорректное тело запроса"
// @Failure      401   {object}  ErrorResponse  "Неверный логин или пароль"
// @Failure      500   {object}  ErrorResponse
// @Router       /api/auth/login [post]
func (h *Handler) LoginAPI(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Одно и то же сообщение для "нет такого пользователя" и "неверный пароль":
	// так нельзя перебором выяснить, какие логины существуют.
	badCredentials := errors.New("неверный логин или пароль")

	user, err := h.Repository.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.errorHandler(ctx, http.StatusUnauthorized, badCredentials)
			return
		}
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		h.errorHandler(ctx, http.StatusUnauthorized, badCredentials)
		return
	}

	token, err := h.Tokens.Generate(user.ID, user.Username)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.Tokens.TTL() / time.Second),
	})
}

// LogoutAPI godoc
// @Summary      Деавторизация
// @Description  Добавляет текущий токен в blacklist (Redis) до конца срока его действия: после этого любой запрос с этим токеном получает 401.
// @Tags         Пользователи
// @Success      200  "Токен отозван, тела ответа нет"
// @Failure      401  {object}  ErrorResponse  "Нет токена, токен просрочен или уже отозван"
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/auth/logout [post]
func (h *Handler) LogoutAPI(ctx *gin.Context) {
	// Middleware AuthRequired уже проверил токен и положил его данные в контекст.
	claims := ctx.MustGet(ctxClaims).(*auth.Claims)

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 { // запись живет ровно столько, сколько токен ещё мог бы работать
		if err := h.Redis.RevokeJWT(ctx.Request.Context(), claims.ID, claims.UserID, ttl); err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	logrus.Infof("logout: user_id=%d, jti=%s", claims.UserID, claims.ID)
	ctx.Status(http.StatusOK)
}
