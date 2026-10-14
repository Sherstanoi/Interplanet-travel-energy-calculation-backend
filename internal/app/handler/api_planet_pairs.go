package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"lab1/internal/app/auth"
	"lab1/internal/app/minio"
	"lab1/internal/app/repository"
)

const (
	maxPhotoSize   = 5 << 20  // 5 МБ
	maxVideoSize   = 50 << 20 // 50 МБ (короткое видео)
	maxRequestSize = 60 << 20 // потолок на весь запрос
	maxFormMemory  = 8 << 20  // сколько multipart держим в ОЗУ, остальное на диск
)

// ---------- Ответ клиенту (DTO) ----------
// IsMine и LikedByMe - указатели с omitempty: nil -> поля нет в JSON, указатель на 0 -> выводится 0.
type planetPairResponse struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	PhotoURL     string    `json:"photo_url"`
	VideoURL     string    `json:"video_url"`
	Distance     int       `json:"distance"`
	Period       int       `json:"period"`
	CreationTime time.Time `json:"creation_time"`
	FormingTime  time.Time `json:"forming_time"`
	CreatorID    int       `json:"creator_id"`
	LikeCount    int       `json:"like_count"`
	IsMine       *int      `json:"is_mine,omitempty"`     // только в обычном списке
	LikedByMe    *int      `json:"liked_by_me,omitempty"` // только в ленте
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intPtr(v int) *int { return &v }

// mediaURL превращает имя объекта из БД во временную ссылку на MinIO.
func mediaURL(objectName string) string {
	if objectName == "" {
		return ""
	}
	u, err := minio.GetPhotoURL(objectName)
	if err != nil {
		logrus.Error("не удалось получить ссылку на файл: ", err)
		return ""
	}
	return u
}

// buildResponses собирает DTO. withIsMine / withLiked включают необязательные поля.
func (h *Handler) buildResponses(pairs []repository.PlanetPair, withIsMine, withLiked bool) []planetPairResponse {
	userID := auth.CurrentUserID()

	ids := make([]int, 0, len(pairs))
	for _, p := range pairs {
		ids = append(ids, p.ID)
	}
	likeCounts, err := h.Repository.GetLikeCounts(ids)
	if err != nil {
		logrus.Error("ошибка подсчета лайков: ", err)
		likeCounts = map[int]int{}
	}

	liked := map[int]bool{}
	if withLiked {
		liked, err = h.Repository.GetLikedByUser(userID, ids)
		if err != nil {
			logrus.Error("ошибка получения лайков пользователя: ", err)
			liked = map[int]bool{}
		}
	}

	out := make([]planetPairResponse, 0, len(pairs))
	for _, p := range pairs {
		item := planetPairResponse{
			ID:           p.ID,
			Name:         p.Name,
			Description:  p.Description,
			Status:       p.Status,
			PhotoURL:     mediaURL(p.PhotoURL),
			VideoURL:     mediaURL(p.VideoURL),
			Distance:     p.Distance,
			Period:       p.Period,
			CreationTime: p.CreationTime,
			FormingTime:  p.FormingTime,
			CreatorID:    p.CreatorID,
			LikeCount:    likeCounts[p.ID],
		}
		if withIsMine {
			item.IsMine = intPtr(b2i(p.CreatorID == userID))
		}
		if withLiked {
			item.LikedByMe = intPtr(b2i(liked[p.ID]))
		}
		out = append(out, item)
	}
	return out
}

// repoError переводит бизнес-ошибки репозитория в HTTP-коды.
func (h *Handler) repoError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		h.errorHandler(ctx, http.StatusNotFound, err)
	case errors.Is(err, repository.ErrForbidden):
		h.errorHandler(ctx, http.StatusForbidden, err)
	case errors.Is(err, repository.ErrWrongStatus), errors.Is(err, repository.ErrDraftExists):
		h.errorHandler(ctx, http.StatusConflict, err)
	default:
		h.errorHandler(ctx, http.StatusInternalServerError, err)
	}
}

func (h *Handler) parseID(ctx *gin.Context) (int, bool) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("id должен быть положительным целым числом"))
		return 0, false
	}
	return id, true
}

// queryInt читает необязательный целый query-параметр >= 0.
func queryInt(ctx *gin.Context, key string) (*int, error) {
	raw := ctx.Query(key)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return nil, fmt.Errorf("параметр %s должен быть неотрицательным целым числом", key)
	}
	return &v, nil
}

// ---------- Работа с файлами ----------
type mediaSpec struct {
	prefix  string
	maxSize int64
	allowed map[string]string // content-type -> расширение
}

var (
	photoSpec = mediaSpec{
		prefix:  "photo",
		maxSize: maxPhotoSize,
		allowed: map[string]string{
			"image/jpeg": ".jpg", "image/png": ".png",
			"image/gif": ".gif", "image/webp": ".webp",
		},
	}
	videoSpec = mediaSpec{
		prefix:  "video",
		maxSize: maxVideoSize,
		allowed: map[string]string{"video/mp4": ".mp4", "video/webm": ".webm"},
	}
)

type validatedFile struct {
	header      *multipart.FileHeader
	contentType string
	ext         string
}

// optionalFile: файла в запросе может не быть (nil, nil); если есть - валидируем по содержимому.
func optionalFile(ctx *gin.Context, field string, spec mediaSpec) (*validatedFile, error) {
	header, err := ctx.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, fmt.Errorf("поле %s: %w", field, err)
	}
	if header.Size > spec.maxSize {
		return nil, fmt.Errorf("поле %s: файл больше %d МБ", field, spec.maxSize>>20)
	}

	f, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("поле %s: не удалось открыть файл", field)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("поле %s: не удалось прочитать файл", field)
	}
	contentType := http.DetectContentType(buf[:n])
	ext, ok := spec.allowed[contentType]
	if !ok {
		return nil, fmt.Errorf("поле %s: недопустимый тип файла %q", field, contentType)
	}
	return &validatedFile{header: header, contentType: contentType, ext: ext}, nil
}

// randomName генерирует латинское имя: photo-3f9a....jpg
func randomName(prefix, ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(b) + ext, nil
}

func saveMedia(ctx context.Context, vf *validatedFile, prefix string) (string, error) {
	name, err := randomName(prefix, vf.ext)
	if err != nil {
		return "", err
	}
	f, err := vf.header.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := minio.UploadObject(ctx, name, f, vf.header.Size, vf.contentType); err != nil {
		return "", err
	}
	return name, nil
}

// ---------- Обработчики ----------

// GET /api/planet-pairs?range_min=&range_max=
// В элементах есть is_mine, liked_by_me нет.
func (h *Handler) ListPlanetPairsAPI(ctx *gin.Context) {
	minRange, err := queryInt(ctx, "range_min")
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}
	maxRange, err := queryInt(ctx, "range_max")
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}
	if minRange != nil && maxRange != nil && *minRange > *maxRange {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("range_min не может быть больше range_max"))
		return
	}

	pairs, err := h.Repository.GetPublishedFiltered(minRange, maxRange)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses(pairs, true, false))
}

// GET /api/planet-pairs/feed                 - вся лента (массив)
// GET /api/planet-pairs/feed?next=true&id=3  - одна карточка, следующая за id=3
// В элементах есть liked_by_me, is_mine нет.
func (h *Handler) FeedPlanetPairsAPI(ctx *gin.Context) {
	if raw := ctx.Query("next"); raw != "" {
		next, err := strconv.ParseBool(raw)
		if err != nil {
			h.errorHandler(ctx, http.StatusBadRequest, errors.New("next должен быть true или false"))
			return
		}
		if next {
			afterID := 0 // без id отдаём первую карточку
			cur, err := queryInt(ctx, "id")
			if err != nil {
				h.errorHandler(ctx, http.StatusBadRequest, err)
				return
			}
			if cur != nil {
				afterID = *cur
			}
			pp, err := h.Repository.GetNextPublished(afterID)
			if err != nil {
				h.repoError(ctx, err)
				return
			}
			ctx.JSON(http.StatusOK, h.buildResponses([]repository.PlanetPair{*pp}, false, true)[0])
			return
		}
	}

	pairs, err := h.Repository.GetPublishedFeed()
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses(pairs, false, true))
}

// GET /api/planet-pairs/draft - id не указывается, черновик определяется по пользователю.
func (h *Handler) GetDraftAPI(ctx *gin.Context) {
	draft, err := h.Repository.GetDraftByCreator(auth.CurrentUserID())
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	if draft == nil {
		h.errorHandler(ctx, http.StatusNotFound, errors.New("черновик не найден"))
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses([]repository.PlanetPair{*draft}, false, false)[0])
}

// POST /api/planet-pairs (multipart/form-data)
// Задаются ТОЛЬКО: name*, photo, video. Остальное заполняет PUT.
// Системные поля (id, status, creator_id, даты) из запроса НЕ читаются.
func (h *Handler) CreatePlanetPairAPI(ctx *gin.Context) {
	userID := auth.CurrentUserID()

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxRequestSize)
	if err := ctx.Request.ParseMultipartForm(maxFormMemory); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	name := strings.TrimSpace(ctx.PostForm("name"))
	if name == "" {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("поле name обязательно"))
		return
	}

	// Не тратим время на загрузку файлов, если черновик уже есть.
	existing, err := h.Repository.GetDraftByCreator(userID)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	if existing != nil {
		h.errorHandler(ctx, http.StatusConflict, repository.ErrDraftExists)
		return
	}

	photo, err := optionalFile(ctx, "photo", photoSpec)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}
	video, err := optionalFile(ctx, "video", videoSpec)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	now := time.Now()
	pp := repository.PlanetPair{
		Name:         name,
		Status:       repository.StatusDraft, // статус и создателя ставит бэкенд
		CreationTime: now,
		FormingTime:  now,
		CreatorID:    userID,
	}

	// Если дальше что-то упадет, загруженные в MinIO файлы удаляем.
	var uploaded []string
	cleanup := func() {
		for _, objName := range uploaded {
			if err := minio.RemoveObject(context.Background(), objName); err != nil {
				logrus.Error("не удалось удалить файл при откате: ", err)
			}
		}
	}

	if photo != nil {
		objName, err := saveMedia(ctx.Request.Context(), photo, photoSpec.prefix)
		if err != nil {
			cleanup()
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
		uploaded = append(uploaded, objName)
		pp.PhotoURL = objName // в БД - только имя файла
	}
	if video != nil {
		objName, err := saveMedia(ctx.Request.Context(), video, videoSpec.prefix)
		if err != nil {
			cleanup()
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
		uploaded = append(uploaded, objName)
		pp.VideoURL = objName
	}

	if err := h.Repository.CreateDraft(&pp); err != nil {
		cleanup()
		h.repoError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, h.buildResponses([]repository.PlanetPair{pp}, false, false)[0])
}

// publishRequest - тело PUT: все поля, которых нет у POST. Все обязательны.
type publishRequest struct {
	Description string `json:"description" binding:"required"`
	Distance    int    `json:"distance" binding:"required,gt=0"`
	Period      int    `json:"period" binding:"required,gt=0"`
}

// PUT /api/planet-pairs/:id/publish - заполняет остальные поля и делает draft -> published
func (h *Handler) PublishPlanetPairAPI(ctx *gin.Context) {
	id, ok := h.parseID(ctx)
	if !ok {
		return
	}

	var req publishRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}
	in := repository.PublishInput{
		Description: strings.TrimSpace(req.Description),
		Distance:    req.Distance,
		Period:      req.Period,
	}
	if in.Description == "" {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("description не может быть пустым"))
		return
	}

	pp, err := h.Repository.PublishPlanetPair(id, auth.CurrentUserID(), in)
	if err != nil {
		h.repoError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses([]repository.PlanetPair{*pp}, false, false)[0])
}

// DELETE /api/planet-pairs/:id - soft delete, ответ 204 без тела
func (h *Handler) DeletePlanetPairAPI(ctx *gin.Context) {
	id, ok := h.parseID(ctx)
	if !ok {
		return
	}
	if err := h.Repository.SoftDeletePlanetPair(id, auth.CurrentUserID()); err != nil {
		h.repoError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

type likeRequest struct {
	// Указатель, чтобы значение 0 не считалось "поле не передано".
	Like *int `json:"like" binding:"required"`
}

// POST /api/planet-pairs/:id/like  body: {"like": 1} или {"like": 0}
func (h *Handler) LikePlanetPairAPI(ctx *gin.Context) {
	id, ok := h.parseID(ctx)
	if !ok {
		return
	}
	var req likeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}
	if *req.Like != 0 && *req.Like != 1 {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("like должен быть 0 или 1"))
		return
	}

	count, err := h.Repository.SetLike(auth.CurrentUserID(), id, *req.Like == 1)
	if err != nil {
		h.repoError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"like_count": count})
}
