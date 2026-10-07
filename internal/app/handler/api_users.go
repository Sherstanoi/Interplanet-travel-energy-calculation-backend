package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"lab1/internal/app/ds"
	"lab1/internal/app/repository"
)

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=72"` // 72 - предел bcrypt
}

// POST /api/users - регистрация
func (h *Handler) RegisterUserAPI(ctx *gin.Context) {
	var req registerRequest
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

	// Хеш пароля наружу не отдаем: возвращаем только нужные поля.
	ctx.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data":   gin.H{"id": user.ID, "username": user.Username},
	})
}

// POST /api/auth/login - заглушка для ЛР4
func (h *Handler) LoginAPI(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "заглушка: аутентификация будет реализована в ЛР4",
	})
}

// POST /api/auth/logout - заглушка для ЛР4
func (h *Handler) LogoutAPI(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "заглушка: деавторизация будет реализована в ЛР4",
	})
}
