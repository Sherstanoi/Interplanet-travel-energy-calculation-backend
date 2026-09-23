package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"lab1/internal/app/minio"
	"lab1/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	defaultPhotoURL = "/static/img/defaultPhotoURL.png"
	defaultVideoURL = "/static/img/defaultVideoURL.mp4"
	draftCreatorID  = 1
)

func (h *Handler) GetPlanetPairs(ctx *gin.Context) {
	var planetPairs []repository.PlanetPair
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

	if minRange > maxRange {
		minRange, maxRange = maxRange, minRange
	}

	if minStr == "" && maxStr == "" {
		planetPairs, err = h.Repository.GetPublishedPlanetPairs()
	} else {
		planetPairs, err = h.Repository.GetPlanetPairsByRange(minRange, maxRange)
	}
	if err != nil {
		logrus.Error(err)
	}

	type PlanetPairWithImage struct {
		repository.PlanetPair
		PhotoURL  string
		VideoURL  string
		LikeCount int
	}

	ids := make([]int, 0, len(planetPairs))
	for _, planetPair := range planetPairs {
		ids = append(ids, planetPair.ID)
	}

	likeCounts, err := h.Repository.GetLikeCounts(ids)
	if err != nil {
		logrus.Error("Ошибка получения лайков:", err)
		likeCounts = map[int]int{}
	}

	planetPairsWithImages := make([]PlanetPairWithImage, 0, len(planetPairs))
	for _, planetPair := range planetPairs {
		imageURL := defaultPhotoURL
		if planetPair.PhotoURL != "" {
			if u, err := minio.GetPhotoURL(planetPair.PhotoURL); err == nil {
				imageURL = u
			} else {
				logrus.Error("Ошибка получения URL изображения:", err)
			}
		}

		videoURL := defaultVideoURL
		if planetPair.VideoURL != "" {
			if u, err := minio.GetPhotoURL(planetPair.VideoURL); err == nil {
				videoURL = u
			} else {
				logrus.Error("Ошибка получения URL видео:", err)
			}
		}

		planetPairsWithImages = append(planetPairsWithImages, PlanetPairWithImage{
			PlanetPair: planetPair,
			PhotoURL:   imageURL,
			VideoURL:   videoURL,
			LikeCount:  likeCounts[planetPair.ID],
		})
	}

	ctx.HTML(http.StatusOK, "plates.html", gin.H{
		"PlanetPairs": planetPairsWithImages,
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

	planetPair, err := h.Repository.GetPublishedPlanetPairByID(id)
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	if ctx.Query("next") == "true" {
		planetPair, err = h.Repository.GetNextPublishedPlanetPair(id)
		if err != nil {
			logrus.Error(err)
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	imageURL := defaultPhotoURL
	if planetPair.PhotoURL != "" {
		if u, err := minio.GetPhotoURL(planetPair.PhotoURL); err == nil {
			imageURL = u
		} else {
			logrus.Error("Ошибка получения URL изображения:", err)
		}
	}

	videoURL := defaultVideoURL
	if planetPair.VideoURL != "" {
		if u, err := minio.GetPhotoURL(planetPair.VideoURL); err == nil {
			videoURL = u
		} else {
			logrus.Error("Ошибка получения URL видео:", err)
		}
	}

	likeCount, err := h.Repository.GetLikeCountByPairID(planetPair.ID)
	if err != nil {
		logrus.Error("Ошибка подсчёта лайков:", err)
		likeCount = 0
	}

	ctx.HTML(http.StatusOK, "vybes.html", gin.H{
		"PlanetPair": planetPair,
		"PhotoURL":   imageURL,
		"VideoURL":   videoURL,
		"LikeCount":  likeCount,
	})
}

func (h *Handler) GetPlanetPairDraft(ctx *gin.Context) {
	draft, err := h.Repository.GetDraftByCreator(draftCreatorID)
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if draft == nil {
		ctx.HTML(http.StatusOK, "create.html", gin.H{
			"IsDraft": false,
		})
		return
	}

	photoURL := defaultPhotoURL
	if draft.PhotoURL != "" {
		if u, err := minio.GetPhotoURL(draft.PhotoURL); err == nil {
			photoURL = u
		} else {
			logrus.Error("Ошибка получения URL изображения:", err)
		}
	}

	videoURL := defaultVideoURL
	if draft.VideoURL != "" {
		if u, err := minio.GetPhotoURL(draft.VideoURL); err == nil {
			videoURL = u
		} else {
			logrus.Error("Ошибка получения URL видео:", err)
		}
	}

	ctx.HTML(http.StatusOK, "create.html", gin.H{
		"IsDraft":    true,
		"PlanetPair": draft,
		"PhotoURL":   photoURL,
		"VideoURL":   videoURL,
	})
}

func (h *Handler) CreatePlanetPair(ctx *gin.Context) {
	planetStart := strings.TrimSpace(ctx.PostForm("PlanetStart"))
	planetEnd := strings.TrimSpace(ctx.PostForm("PlanetEnd"))

	var validationErrors []string
	if planetStart == "" {
		validationErrors = append(validationErrors, "Укажите планету-отправителя")
	}
	if planetEnd == "" {
		validationErrors = append(validationErrors, "Укажите планету-получателя")
	}

	if len(validationErrors) > 0 {
		ctx.HTML(http.StatusOK, "create.html", gin.H{
			"IsDraft":         false,
			"Errors":          validationErrors,
			"FormPlanetStart": planetStart,
			"FormPlanetEnd":   planetEnd,
		})
		return
	}

	pp := repository.PlanetPair{
		PlanetStart:  planetStart,
		PlanetEnd:    planetEnd,
		Status:       "draft",
		CreationTime: time.Now(),
		FormingTime:  time.Now(),
		CreatorID:    draftCreatorID,
	}

	if err := h.Repository.CreatePlanetPair(&pp); err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	ctx.Redirect(http.StatusSeeOther, "/planet_pairs/create")
}

func (h *Handler) PublishPlanetPair(ctx *gin.Context) {
	draft, err := h.Repository.GetDraftByCreator(draftCreatorID)
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if draft == nil {
		logrus.Error("черновик не найден")
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	description := strings.TrimSpace(ctx.PostForm("Description"))
	distanceStr := strings.TrimSpace(ctx.PostForm("Distance"))
	periodStr := strings.TrimSpace(ctx.PostForm("Period"))

	var validationErrors []string

	distance, errDist := strconv.Atoi(distanceStr)
	if errDist != nil || distance <= 0 {
		validationErrors = append(validationErrors, "Расстояние должно быть положительным числом")
	}

	period, errPer := strconv.Atoi(periodStr)
	if errPer != nil || period <= 0 {
		validationErrors = append(validationErrors, "Период сближения должен быть положительным числом")
	}

	if description == "" {
		validationErrors = append(validationErrors, "Заполните дополнительную информацию")
	}

	if len(validationErrors) > 0 {
		draft.Description = description
		if errDist == nil {
			draft.Distance = distance
		}
		if errPer == nil {
			draft.Period = period
		}

		photoURL := defaultPhotoURL
		if draft.PhotoURL != "" {
			if u, err := minio.GetPhotoURL(draft.PhotoURL); err == nil {
				photoURL = u
			}
		}

		videoURL := defaultVideoURL
		if draft.VideoURL != "" {
			if u, err := minio.GetPhotoURL(draft.VideoURL); err == nil {
				videoURL = u
			}
		}

		ctx.HTML(http.StatusOK, "create.html", gin.H{
			"IsDraft":    true,
			"PlanetPair": draft,
			"PhotoURL":   photoURL,
			"VideoURL":   videoURL,
			"Errors":     validationErrors,
		})
		return
	}

	draft.Description = description
	draft.Distance = distance
	draft.Period = period
	draft.Status = "published"
	draft.FormingTime = time.Now()

	if err := h.Repository.UpdatePlanetPair(draft); err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	ctx.Redirect(http.StatusSeeOther, "/planet_pairs")
}

func (h *Handler) DeletePlanetPair(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err := h.Repository.DeletePlanetPairByID(id); err != nil {
		logrus.Error(err)
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	ctx.Redirect(http.StatusSeeOther, "/planet_pairs")
}
