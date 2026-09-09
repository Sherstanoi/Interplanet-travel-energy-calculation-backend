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

func (h *Handler) GetCards(ctx *gin.Context) {
	var cards []repository.Card
	var err error

	massStr := ctx.Query("mass")
	if massStr == "" {
		cards, err = h.Repository.GetPublishedCards()
	} else {
		cards, err = h.Repository.GetCardsByMass(massStr)
	}

	if err != nil {
		logrus.Error(err)
	}

	type CardWithImage struct {
		repository.Card
		ImageURL string
		VideoURL string
	}

	cardsWithImages := make([]CardWithImage, 0, len(cards))
	for _, card := range cards {
		imageURL := ""
		videoURL := ""

		if card.ImageURL != "" {
			imageURL, err = minio.GetImageURL(card.ImageURL)
			if err != nil {
				logrus.Error("Ошибка получения URL изображения:", err)
				imageURL = ""
			}
		}

		if card.VideoURL != "" {
			videoURL, err = minio.GetImageURL(card.VideoURL)
			if err != nil {
				logrus.Error("Ошибка получения URL видео:", err)
				videoURL = ""
			}
		}

		cardsWithImages = append(cardsWithImages, CardWithImage{
			Card:     card,
			ImageURL: imageURL,
			VideoURL: videoURL,
		})
	}

	ctx.HTML(http.StatusOK, "plates.html", gin.H{
		"Cards": cardsWithImages,
		"mass":  massStr,
	})
}

func (h *Handler) GetCard(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	publishedCards, err := h.Repository.GetPublishedCards()
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	currentIndex := -1
	for i, card := range publishedCards {
		if card.ID == id {
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
		if currentIndex+1 < len(publishedCards) {
			currentIndex++
		} else {
			currentIndex = 0
		}
	}

	currentCard := publishedCards[currentIndex]

	imageURL, err := minio.GetImageURL(currentCard.ImageURL)
	if err != nil {
		logrus.Error("Ошибка получения URL изображения:", err)
		imageURL = ""
	}

	videoURL, err := minio.GetImageURL(currentCard.VideoURL)
	if err != nil {
		logrus.Error("Ошибка получения URL видео:", err)
		videoURL = ""
	}

	ctx.HTML(http.StatusOK, "vybes.html", gin.H{
		"Card":     currentCard,
		"ImageURL": imageURL,
		"VideoURL": videoURL,
	})
}

func (h *Handler) GetDraft(ctx *gin.Context) {
	card, err := h.Repository.GetDraft()
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	imageURL, err := minio.GetImageURL(card.ImageURL)
	if err != nil {
		logrus.Error("Ошибка получения URL изображения:", err)
		imageURL = ""
	}

	videoURL, err := minio.GetImageURL(card.VideoURL)
	if err != nil {
		logrus.Error("Ошибка получения URL видео:", err)
		videoURL = ""
	}

	ctx.HTML(http.StatusOK, "create.html", gin.H{
		"Card":     card,
		"ImageURL": imageURL,
		"VideoURL": videoURL,
	})
}
