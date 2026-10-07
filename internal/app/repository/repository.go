package repository

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

type PlanetPair struct {
	ID           int       `gorm:"column:planetpairs_id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Description  string    `gorm:"column:description"`
	Status       string    `gorm:"column:status"`
	PhotoURL     string    `gorm:"column:photo_url"`
	VideoURL     string    `gorm:"column:video_url"`
	Distance     int       `gorm:"column:distance"`
	Period       int       `gorm:"column:period"`
	CreationTime time.Time `gorm:"column:creation_time"`
	FormingTime  time.Time `gorm:"column:forming_time"`
	CreatorID    int       `gorm:"column:creator_id"`
}

func (PlanetPair) TableName() string {
	return "planet_pairs"
}
