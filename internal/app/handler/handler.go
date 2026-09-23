package handler

import (
	"lab1/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

const (
	defaultRangeMin = 0
	defaultRangeMax = 500000
)

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

func (h *Handler) RegisterHandler(router *gin.Engine) {
	router.GET("/planet_pairs", h.GetPlanetPairs)
	router.GET("/planet_pairs/create", h.GetPlanetPairDraft)
	router.GET("/planet_pairs/:id", h.GetPlanetPair)
	router.POST("/planet_pairs/create", h.CreatePlanetPair)
	router.POST("/planet_pairs/publish", h.PublishPlanetPair)
	router.POST("/planet_pairs/:id/delete", h.DeletePlanetPair)
}

func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

func (h *Handler) errorHandler(ctx *gin.Context, errorStatusCode int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(errorStatusCode, gin.H{
		"status":      "error",
		"description": err.Error(),
	})
}
