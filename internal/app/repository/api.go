package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"lab1/internal/app/ds"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Статусы услуги. Допустимые переходы:
// draft -> published, draft -> deleted, published -> deleted.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusDeleted   = "deleted"
)

// Бизнес-ошибки: handler переводит их в HTTP-коды.
var (
	ErrNotFound      = errors.New("запись не найдена")
	ErrForbidden     = errors.New("действие разрешено только создателю")
	ErrWrongStatus   = errors.New("недопустимый переход статуса")
	ErrDraftExists   = errors.New("у пользователя уже есть черновик")
	ErrIncomplete    = errors.New("не заполнены обязательные поля для публикации")
	ErrUsernameTaken = errors.New("имя пользователя уже занято")
)

// GetPublishedFiltered - список опубликованных с фильтром по расстоянию.
// nil означает "ограничения нет".
func (r *Repository) GetPublishedFiltered(minRange, maxRange *int) ([]PlanetPair, error) {
	q := r.db.Model(&PlanetPair{}).Where("status ILIKE ?", StatusPublished)
	if minRange != nil {
		q = q.Where("distance >= ?", *minRange)
	}
	if maxRange != nil {
		q = q.Where("distance <= ?", *maxRange)
	}
	var res []PlanetPair
	if err := q.Order("planetpairs_id ASC").Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// GetPublishedFeed - лента: новые сверху, с пагинацией.
func (r *Repository) GetPublishedFeed(limit, offset int) ([]PlanetPair, error) {
	var res []PlanetPair
	err := r.db.
		Where("status ILIKE ?", StatusPublished).
		Order("forming_time DESC, planetpairs_id DESC").
		Limit(limit).Offset(offset).
		Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

// CreateDraft создает услугу-черновик; у пользователя не более одного черновика.
func (r *Repository) CreateDraft(pp *PlanetPair) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var cnt int64
		err := tx.Model(&PlanetPair{}).
			Where("creator_id = ? AND status ILIKE ?", pp.CreatorID, StatusDraft).
			Count(&cnt).Error
		if err != nil {
			return err
		}
		if cnt > 0 {
			return ErrDraftExists
		}
		return tx.Create(pp).Error
	})
}

// getAlive возвращает услугу, если она не удалена.
func (r *Repository) getAlive(tx *gorm.DB, id int) (*PlanetPair, error) {
	var pp PlanetPair
	err := tx.Where("planetpairs_id = ? AND status NOT ILIKE ?", id, StatusDeleted).First(&pp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &pp, nil
}

// PublishPlanetPair: draft -> published, только создатель, только полностью заполненный.
func (r *Repository) PublishPlanetPair(id, userID int) (*PlanetPair, error) {
	var result *PlanetPair
	err := r.db.Transaction(func(tx *gorm.DB) error {
		pp, err := r.getAlive(tx, id)
		if err != nil {
			return err
		}
		if pp.CreatorID != userID {
			return ErrForbidden
		}
		if !strings.EqualFold(pp.Status, StatusDraft) {
			return fmt.Errorf("%w: из статуса %q публикация невозможна", ErrWrongStatus, pp.Status)
		}
		if strings.TrimSpace(pp.Description) == "" || pp.Distance <= 0 || pp.Period <= 0 {
			return fmt.Errorf("%w (description, distance > 0, period > 0)", ErrIncomplete)
		}

		now := time.Now()
		err = tx.Model(&PlanetPair{}).
			Where("planetpairs_id = ?", id).
			Updates(map[string]any{"status": StatusPublished, "forming_time": now}).Error
		if err != nil {
			return err
		}
		pp.Status = StatusPublished
		pp.FormingTime = now
		result = pp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SoftDeletePlanetPair: draft|published -> deleted, только услуги этого пользователя.
func (r *Repository) SoftDeletePlanetPair(id, userID int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		pp, err := r.getAlive(tx, id)
		if err != nil {
			return err
		}
		if pp.CreatorID != userID {
			return ErrForbidden
		}
		return tx.Model(&PlanetPair{}).
			Where("planetpairs_id = ?", id).
			Update("status", StatusDeleted).Error
	})
}

// SetLike ставит (like=true) или снимает (like=false) лайк. Возвращает число лайков.
func (r *Repository) SetLike(userID, pairID int, like bool) (int, error) {
	var cnt int64
	err := r.db.Model(&PlanetPair{}).
		Where("planetpairs_id = ? AND status ILIKE ?", pairID, StatusPublished).
		Count(&cnt).Error
	if err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, ErrNotFound // лайкать можно только опубликованные
	}

	if like {
		// Повторный лайк не ошибка: ON CONFLICT DO NOTHING (уникальный индекс user+pair).
		err = r.db.
			Omit(clause.Associations).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&ds.Like{PlanetPairID: pairID, UserID: userID}).Error
	} else {
		err = r.db.
			Where("planetpair_id = ? AND user_id = ?", pairID, userID).
			Delete(&ds.Like{}).Error
	}
	if err != nil {
		return 0, err
	}
	return r.GetLikeCountByPairID(pairID) // уже есть в вашем order.go
}

// GetLikedByUser возвращает множество id услуг, которые лайкнул пользователь.
func (r *Repository) GetLikedByUser(userID int, pairIDs []int) (map[int]bool, error) {
	res := make(map[int]bool)
	if len(pairIDs) == 0 {
		return res, nil
	}
	var ids []int
	err := r.db.Model(&ds.Like{}).
		Where("user_id = ? AND planetpair_id IN ?", userID, pairIDs).
		Pluck("planetpair_id", &ids).Error
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		res[id] = true
	}
	return res, nil
}

// CreateUser - регистрация (пароль уже захеширован в handler).
func (r *Repository) CreateUser(u *ds.User) error {
	var cnt int64
	if err := r.db.Model(&ds.User{}).Where("username = ?", u.Username).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return ErrUsernameTaken
	}
	return r.db.Create(u).Error
}
