package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"lab1/internal/app/ds"

	"gorm.io/gorm"
)

func (r *Repository) GetPlanetPairs() ([]PlanetPair, error) {
	var planetPairs []PlanetPair
	err := r.db.Find(&planetPairs).Error
	if err != nil {
		return nil, err
	}
	if len(planetPairs) == 0 {
		return nil, fmt.Errorf("массив пустой")
	}
	return planetPairs, nil
}

func (r *Repository) GetPublishedPlanetPairs() ([]PlanetPair, error) {
	var planetPairs []PlanetPair
	err := r.db.Where("status ILIKE ?", "published").Find(&planetPairs).Error
	if err != nil {
		return nil, err
	}
	if len(planetPairs) == 0 {
		return nil, fmt.Errorf("массив пустой")
	}
	return planetPairs, nil
}

func (r *Repository) GetPlanetPairsByRange(minRange, maxRange int) ([]PlanetPair, error) {
	var planetPairs []PlanetPair
	err := r.db.
		Where("status ILIKE ?", "published").
		Where("distance >= ?", minRange).
		Where("distance <= ?", maxRange).
		Find(&planetPairs).Error
	if err != nil {
		return nil, err
	}
	return planetPairs, nil
}

func (r *Repository) GetPlanetPairDraft() (PlanetPair, error) {
	planetPair := PlanetPair{}
	err := r.db.Where("status ILIKE ?", "draft").First(&planetPair).Error
	if err != nil {
		return PlanetPair{}, fmt.Errorf("нет черновика")
	}
	return planetPair, nil
}

func (r *Repository) GetPublishedPlanetPairByID(id int) (PlanetPair, error) {
	planetPair := PlanetPair{}
	err := r.db.
		Where("planetpairs_id = ? AND status ILIKE ?", id, "published").
		First(&planetPair).Error
	if err != nil {
		return PlanetPair{}, err
	}
	return planetPair, nil
}

func (r *Repository) GetNextPublishedPlanetPair(id int) (PlanetPair, error) {
	planetPair := PlanetPair{}
	err := r.db.
		Where("planetpairs_id > ? AND status ILIKE ?", id, "published").
		Order("planetpairs_id ASC").
		First(&planetPair).Error
	if err != nil {
		err = r.db.
			Where("status ILIKE ?", "published").
			Order("planetpairs_id ASC").
			First(&planetPair).Error
		if err != nil {
			return PlanetPair{}, err
		}
	}
	return planetPair, nil
}

func (r *Repository) GetDraftByCreator(creatorID int) (*PlanetPair, error) {
	var pp PlanetPair
	err   := r.db.
		Where("creator_id = ? AND status = ?", creatorID, "draft").
		First(&pp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pp, nil
}

func (r *Repository) CreatePlanetPair(pp *PlanetPair) error {
	return r.db.Create(pp).Error
}

func (r *Repository) UpdatePlanetPair(pp *PlanetPair) error {
	return r.db.Save(pp).Error
}

func (r *Repository) DeletePlanetPairByID(id int) error {
	query := `UPDATE planet_pairs
	          SET status = 'deleted'
	          WHERE planetpairs_id = $1
	          RETURNING planetpairs_id`

	row := r.db.Raw(query, id).Row()

	var deletedID int
	if err := row.Scan(&deletedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("запись с id=%d не найдена", id)
		}
		return err
	}
	return nil
}

func (r *Repository) GetLikeCounts(planetPairIDs []int) (map[int]int, error) {
	result := make(map[int]int)

	if len(planetPairIDs) == 0 {
		return result, nil
	}

	type row struct {
		PlanetPairID int `gorm:"column:planetpair_id"`
		Cnt          int `gorm:"column:cnt"`
	}
	var rows []row

	err := r.db.
		Model(&ds.Like{}).
		Select("planetpair_id, COUNT(*) AS cnt").
		Where("planetpair_id IN ?", planetPairIDs).
		Group("planetpair_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.PlanetPairID] = row.Cnt
	}
	return result, nil
}

func (r *Repository) GetLikeCountByPairID(planetPairID int) (int, error) {
	var cnt int64
	err := r.db.
		Model(&ds.Like{}).
		Where("planetpair_id = ?", planetPairID).
		Count(&cnt).Error
	if err != nil {
		return 0, err
	}
	return int(cnt), nil
}
