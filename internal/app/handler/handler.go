package handler

import (
	"lab1/internal/app/minio"
	"lab1/internal/app/repository"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

// Границы слайдеров (совпадают с диапазоном Distance в данных).
const (
	defaultRangeMin = 0
	defaultRangeMax = 500000
)

func (h *Handler) GetPlanetPairs(ctx *gin.Context) {
	var PlanetPairs []repository.PlanetPair
	var err error

	minStr := ctx.Query("range_min")
	maxStr := ctx.Query("range_max")

	minRange := defaultRangeMin
	maxRange := defaultRangeMax

	if minStr != "" {
		if v, convErr := strconv.Atoi(minStr); convErr == nil {
			minRange = v
		}
	}
	if maxStr != "" {
		if v, convErr := strconv.Atoi(maxStr); convErr == nil {
			maxRange = v
		}
	}

	// Если пользователь перепутал слайдеры — меняем местами.
	if minRange > maxRange {
		minRange, maxRange = maxRange, minRange
	}

	// Если фильтр не задан вообще — показываем всё.
	if minStr == "" && maxStr == "" {
		PlanetPairs, err = h.Repository.GetPublishedPlanetPairs()
	} else {
		PlanetPairs, err = h.Repository.GetPlanetPairsByRange(minRange, maxRange)
	}

	if err != nil {
		logrus.Error(err)
	}

	type PlanetPairWithImage struct {
		repository.PlanetPair
		ImageURL string
		VideoURL string
	}

	PlanetPairsWithImages := make([]PlanetPairWithImage, 0, len(PlanetPairs))
	for _, PlanetPair := range PlanetPairs {
		imageURL := ""
		videoURL := ""

		if PlanetPair.ImageURL != "" {
			imageURL, err = minio.GetImageURL(PlanetPair.ImageURL)
			if err != nil {
				logrus.Error("Ошибка получения URL изображения:", err)
				imageURL = ""
			}
		}

		if PlanetPair.VideoURL != "" {
			videoURL, err = minio.GetImageURL(PlanetPair.VideoURL)
			if err != nil {
				logrus.Error("Ошибка получения URL видео:", err)
				videoURL = ""
			}
		}

		PlanetPairsWithImages = append(PlanetPairsWithImages, PlanetPairWithImage{
			PlanetPair: PlanetPair,
			ImageURL:   imageURL,
			VideoURL:   videoURL,
		})
	}

	ctx.HTML(http.StatusOK, "plates.html", gin.H{
		"PlanetPairs": PlanetPairsWithImages,
		"range_min":   minRange,
		"range_max":   maxRange,
	})
}

func (h *Handler) GetPlanetPair(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	publishedPlanetPairs, err := h.Repository.GetPublishedPlanetPairs()
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	currentIndex := -1
	for i, PlanetPair := range publishedPlanetPairs {
		if PlanetPair.ID == id {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	nextParam := ctx.Query("next")
	if nextParam == "true" {
		if currentIndex+1 < len(publishedPlanetPairs) {
			currentIndex++
		} else {
			currentIndex = 0
		}
	}

	currentPlanetPair := publishedPlanetPairs[currentIndex]

	imageURL, err := minio.GetImageURL(currentPlanetPair.ImageURL)
	if err != nil {
		logrus.Error("Ошибка получения URL изображения:", err)
		imageURL = ""
	}

	videoURL, err := minio.GetImageURL(currentPlanetPair.VideoURL)
	if err != nil {
		logrus.Error("Ошибка получения URL видео:", err)
		videoURL = ""
	}

	ctx.HTML(http.StatusOK, "vybes.html", gin.H{
		"PlanetPair": currentPlanetPair,
		"ImageURL":   imageURL,
		"VideoURL":   videoURL,
	})
}

func (h *Handler) GetPlanetPairDraft(ctx *gin.Context) {
	PlanetPair, err := h.Repository.GetPlanetPairDraft()
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	imageURL, err := minio.GetImageURL(PlanetPair.ImageURL)
	if err != nil {
		logrus.Error("Ошибка получения URL изображения:", err)
		imageURL = ""
	}

	videoURL, err := minio.GetImageURL(PlanetPair.VideoURL)
	if err != nil {
		logrus.Error("Ошибка получения URL видео:", err)
		videoURL = ""
	}

	ctx.HTML(http.StatusOK, "create.html", gin.H{
		"PlanetPair": PlanetPair,
		"ImageURL":   imageURL,
		"VideoURL":   videoURL,
	})
}
