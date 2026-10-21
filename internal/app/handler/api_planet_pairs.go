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

	"lab1/internal/app/minio"
	"lab1/internal/app/repository"
)

const (
	maxPhotoSize   = 5 << 20  // 5 МБ
	maxVideoSize   = 50 << 20 // 50 МБ (короткое видео)
	maxRequestSize = 60 << 20 // потолок на весь запрос
	maxFormMemory  = 8 << 20  // сколько multipart держим в ОЗУ, остальное на диск
)

// ---------- Ответы и тела запросов (попадают в Swagger) ----------

// PlanetPairResponse - услуга в ответе API.
// is_mine и liked_by_me - указатели с omitempty: nil -> поля нет в JSON, указатель на 0 -> выводится 0.
type PlanetPairResponse struct {
	ID           int       `json:"id" example:"1"`
	Name         string    `json:"name" example:"Земля - Марс"`
	Description  string    `json:"description" example:"Окно запуска"`
	Status       string    `json:"status" enums:"draft,published" example:"published"`
	PhotoURL     string    `json:"photo_url" example:"http://localhost:9000/media/photo-3f9a.jpg?X-Amz-..."`
	VideoURL     string    `json:"video_url" example:"http://localhost:9000/media/video-7c1b.mp4?X-Amz-..."`
	Distance     int       `json:"distance" example:"225000000"`
	Period       int       `json:"period" example:"780"`
	CreationTime time.Time `json:"creation_time"`
	FormingTime  time.Time `json:"forming_time"`
	CreatorID    int       `json:"creator_id" example:"1"`
	LikeCount    int       `json:"like_count" example:"3"`
	IsMine       *int      `json:"is_mine,omitempty" enums:"0,1" example:"1"`     // только в списке
	LikedByMe    *int      `json:"liked_by_me,omitempty" enums:"0,1" example:"0"` // только в ленте
}

// PublishRequest - тело PUT: все поля, которых нет у POST. Все обязательны.
type PublishRequest struct {
	Description string `json:"description" binding:"required" example:"Окно запуска Земля - Марс"`
	Distance    int    `json:"distance" binding:"required,gt=0" example:"225000000"`
	Period      int    `json:"period" binding:"required,gt=0" example:"780"`
}

// LikeRequest - тело лайка: 1 ставит, 0 снимает.
type LikeRequest struct {
	// Указатель, чтобы значение 0 не считалось "поле не передано".
	Like *int `json:"like" binding:"required" enums:"0,1" example:"1"`
}

// LikeResponse - число лайков услуги после операции.
type LikeResponse struct {
	LikeCount int `json:"like_count" example:"4"`
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

// buildResponses собирает DTO. userID = 0 означает гостя: is_mine и liked_by_me будут 0.
func (h *Handler) buildResponses(pairs []repository.PlanetPair, userID int, withIsMine, withLiked bool) []PlanetPairResponse {
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
	if withLiked && userID > 0 {
		liked, err = h.Repository.GetLikedByUser(userID, ids)
		if err != nil {
			logrus.Error("ошибка получения лайков пользователя: ", err)
			liked = map[int]bool{}
		}
	}

	out := make([]PlanetPairResponse, 0, len(pairs))
	for _, p := range pairs {
		item := PlanetPairResponse{
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
			item.IsMine = intPtr(b2i(userID > 0 && p.CreatorID == userID))
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

// ---------- Обработчики: чтение (доступно без авторизации) ----------

// ListPlanetPairsAPI godoc
// @Summary      Список опубликованных услуг
// @Description  Возвращает только опубликованные услуги (черновики и удалённые не передаются), с фильтром по расстоянию.
// @Description  Доступен без авторизации и работает одинаково с любым токеном. Если передан действующий токен, в каждом элементе
// @Description  поле is_mine = 1 для услуг, созданных этим пользователем; для гостя is_mine всегда 0.
// @Tags         Услуги: чтение
// @Produce      json
// @Param        range_min  query     int  false  "Минимальное расстояние (включительно)"  minimum(0)
// @Param        range_max  query     int  false  "Максимальное расстояние (включительно)" minimum(0)
// @Success      200        {array}   PlanetPairResponse
// @Failure      400        {object}  ErrorResponse  "Некорректный фильтр"
// @Failure      500        {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs [get]
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
	ctx.JSON(http.StatusOK, h.buildResponses(pairs, currentUserID(ctx), true, false))
}

// FeedPlanetPairsAPI godoc
// @Summary      Лента опубликованных услуг
// @Description  Без параметров возвращает массив всех опубликованных услуг по возрастанию id.
// @Description  С next=true возвращает ОДНУ услугу (объект, а не массив), следующую за услугой с id; после последней возвращается первая,
// @Description  без id возвращается первая. Доступна без авторизации. В элементах есть liked_by_me: 1, если текущий пользователь
// @Description  (по токену) лайкнул услугу, для гостя всегда 0.
// @Tags         Услуги: чтение
// @Produce      json
// @Param        next  query     bool  false  "true - вернуть одну услугу, следующую за id"
// @Param        id    query     int   false  "id текущей услуги (используется вместе с next=true)"  minimum(0)
// @Success      200   {array}   PlanetPairResponse  "Лента; при next=true в ответе один объект PlanetPairResponse"
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "Нет опубликованных услуг"
// @Failure      500   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs/feed [get]
func (h *Handler) FeedPlanetPairsAPI(ctx *gin.Context) {
	userID := currentUserID(ctx)

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
			ctx.JSON(http.StatusOK, h.buildResponses([]repository.PlanetPair{*pp}, userID, false, true)[0])
			return
		}
	}

	pairs, err := h.Repository.GetPublishedFeed()
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses(pairs, userID, false, true))
}

// ---------- Обработчики: методы создателя (нужен токен) ----------

// GetDraftAPI godoc
// @Summary      Черновик текущего пользователя
// @Description  Возвращает черновик пользователя из токена (не более одного), id не указывается. Только для авторизованных.
// @Tags         Услуги: создатель
// @Produce      json
// @Success      200  {object}  PlanetPairResponse
// @Failure      401  {object}  ErrorResponse  "Нет токена, токен просрочен или отозван"
// @Failure      404  {object}  ErrorResponse  "Черновика нет"
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs/draft [get]
func (h *Handler) GetDraftAPI(ctx *gin.Context) {
	userID := currentUserID(ctx)
	draft, err := h.Repository.GetDraftByCreator(userID)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}
	if draft == nil {
		h.errorHandler(ctx, http.StatusNotFound, errors.New("черновик не найден"))
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses([]repository.PlanetPair{*draft}, userID, false, false)[0])
}

// CreatePlanetPairAPI godoc
// @Summary      Создать услугу (черновик)
// @Description  multipart/form-data: задаются только name, photo и video. Остальные поля заполняются при публикации (PUT).
// @Description  Файлы сохраняются в MinIO под сгенерированными латинскими именами, имена записываются в БД.
// @Description  Создатель услуги заполняется автоматически из токена; id, статус и даты клиентом не задаются.
// @Description  У пользователя может быть не более одного черновика.
// @Tags         Услуги: создатель
// @Accept       mpfd
// @Produce      json
// @Param        name   formData  string  true   "Название"
// @Param        photo  formData  file    false  "Изображение (jpg, png, gif, webp; до 5 МБ)"
// @Param        video  formData  file    false  "Короткое видео (mp4, webm; до 50 МБ)"
// @Success      201    {object}  PlanetPairResponse
// @Failure      400    {object}  ErrorResponse  "Нет name или недопустимый файл"
// @Failure      401    {object}  ErrorResponse  "Нет токена, токен просрочен или отозван"
// @Failure      409    {object}  ErrorResponse  "Черновик уже существует"
// @Failure      500    {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs [post]
func (h *Handler) CreatePlanetPairAPI(ctx *gin.Context) {
	userID := currentUserID(ctx) // автозаполнение создателя: берется из токена

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

	ctx.JSON(http.StatusCreated, h.buildResponses([]repository.PlanetPair{pp}, userID, false, false)[0])
}

// PublishPlanetPairAPI godoc
// @Summary      Опубликовать услугу
// @Description  Заполняет остальные поля (description, distance, period) и меняет статус draft -> published.
// @Description  Только создатель услуги; вернуть услугу в черновик нельзя.
// @Tags         Услуги: создатель
// @Accept       json
// @Produce      json
// @Param        id    path      int             true  "ID услуги"
// @Param        body  body      PublishRequest  true  "Поля услуги"
// @Success      200   {object}  PlanetPairResponse
// @Failure      400   {object}  ErrorResponse  "Некорректный id или поля"
// @Failure      401   {object}  ErrorResponse  "Нет токена, токен просрочен или отозван"
// @Failure      403   {object}  ErrorResponse  "Услуга принадлежит другому пользователю"
// @Failure      404   {object}  ErrorResponse  "Услуга не найдена"
// @Failure      409   {object}  ErrorResponse  "Услуга уже опубликована"
// @Failure      500   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs/{id}/publish [put]
func (h *Handler) PublishPlanetPairAPI(ctx *gin.Context) {
	id, ok := h.parseID(ctx)
	if !ok {
		return
	}

	var req PublishRequest
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

	userID := currentUserID(ctx)
	pp, err := h.Repository.PublishPlanetPair(id, userID, in)
	if err != nil {
		h.repoError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, h.buildResponses([]repository.PlanetPair{*pp}, userID, false, false)[0])
}

// DeletePlanetPairAPI godoc
// @Summary      Удалить услугу
// @Description  Мягкое удаление (статус deleted, запись остаётся в БД). Только услуги текущего пользователя.
// @Tags         Услуги: создатель
// @Param        id   path  int  true  "ID услуги"
// @Success      204  "Удалено, тела ответа нет"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse  "Нет токена, токен просрочен или отозван"
// @Failure      403  {object}  ErrorResponse  "Услуга принадлежит другому пользователю"
// @Failure      404  {object}  ErrorResponse  "Услуга не найдена"
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs/{id} [delete]
func (h *Handler) DeletePlanetPairAPI(ctx *gin.Context) {
	id, ok := h.parseID(ctx)
	if !ok {
		return
	}
	if err := h.Repository.SoftDeletePlanetPair(id, currentUserID(ctx)); err != nil {
		h.repoError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// LikePlanetPairAPI godoc
// @Summary      Поставить или снять лайк
// @Description  like = 1 ставит лайк от текущего пользователя, like = 0 отменяет. Повторный лайк не создаёт дубликат.
// @Description  Лайкать можно только опубликованные услуги.
// @Tags         Услуги: создатель
// @Accept       json
// @Produce      json
// @Param        id    path      int          true  "ID услуги"
// @Param        body  body      LikeRequest  true  "like: 1 или 0"
// @Success      200   {object}  LikeResponse
// @Failure      400   {object}  ErrorResponse  "like не 0/1 или некорректный id"
// @Failure      401   {object}  ErrorResponse  "Нет токена, токен просрочен или отозван"
// @Failure      404   {object}  ErrorResponse  "Опубликованной услуги нет"
// @Failure      500   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/planet-pairs/{id}/like [post]
func (h *Handler) LikePlanetPairAPI(ctx *gin.Context) {
	id, ok := h.parseID(ctx)
	if !ok {
		return
	}
	var req LikeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}
	if *req.Like != 0 && *req.Like != 1 {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("like должен быть 0 или 1"))
		return
	}

	count, err := h.Repository.SetLike(currentUserID(ctx), id, *req.Like == 1)
	if err != nil {
		h.repoError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, LikeResponse{LikeCount: count})
}
