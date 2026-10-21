package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"lab1/internal/app/auth"
	appredis "lab1/internal/app/redis"
	"lab1/internal/app/repository"
)

type Handler struct {
	Repository *repository.Repository
	Redis      *appredis.Client   // blacklist отозванных токенов
	Tokens     *auth.TokenManager // выпуск и проверка JWT
}

const (
	defaultRangeMin = 0
	defaultRangeMax = 500000
)

func NewHandler(r *repository.Repository, rc *appredis.Client, tm *auth.TokenManager) *Handler {
	return &Handler{
		Repository: r,
		Redis:      rc,
		Tokens:     tm,
	}
}

// RegisterHandler - HTML-страницы из ЛР1-2 (без изменений).
func (h *Handler) RegisterHandler(router *gin.Engine) {
	router.GET("/planet_pairs", h.GetPlanetPairs)
	router.GET("/planet_pairs/create", h.GetPlanetPairDraft)
	router.GET("/planet_pairs/:id", h.GetPlanetPair)
	router.POST("/planet_pairs/create", h.CreatePlanetPair)
	router.POST("/planet_pairs/publish", h.PublishPlanetPair)
	router.POST("/planet_pairs/:id/delete", h.DeletePlanetPair)
}

// RegisterAPI - REST API (все адреса начинаются с /api) и страница Swagger.
func (h *Handler) RegisterAPI(router *gin.Engine) {
	api := router.Group("/api")

	// Чтение доступно всем. AuthOptional читает токен, если он есть
	// (нужно для is_mine и liked_by_me), но гостя не отклоняет:
	// даже с неверным токеном эти методы работают как для гостя.
	read := api.Group("/planet-pairs", h.AuthOptional())
	read.GET("", h.ListPlanetPairsAPI)
	read.GET("/feed", h.FeedPlanetPairsAPI)

	// Методы создателя: нужен действующий токен, иначе 401.
	creator := api.Group("/planet-pairs", h.AuthRequired())
	creator.GET("/draft", h.GetDraftAPI)
	creator.POST("", h.CreatePlanetPairAPI)
	creator.PUT("/:id/publish", h.PublishPlanetPairAPI)
	creator.DELETE("/:id", h.DeletePlanetPairAPI)
	creator.POST("/:id/like", h.LikePlanetPairAPI)

	// Пользователи
	api.POST("/users", h.RegisterUserAPI)
	api.POST("/auth/login", h.LoginAPI)
	api.POST("/auth/logout", h.AuthRequired(), h.LogoutAPI)

	// Swagger UI: http://<host>:<port>/swagger/index.html
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

// ErrorResponse - единый формат ошибки.
type ErrorResponse struct {
	Status      string `json:"status" example:"error"`
	Description string `json:"description" example:"описание ошибки"`
}

func (h *Handler) errorHandler(ctx *gin.Context, errorStatusCode int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(errorStatusCode, ErrorResponse{
		Status:      "error",
		Description: err.Error(),
	})
}
