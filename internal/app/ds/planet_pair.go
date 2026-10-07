package ds

import "time"

type PlanetPair struct {
	ID int `gorm:"column:planetpairs_id;primaryKey;autoIncrement"`
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

	Creator User `gorm:"foreignKey:CreatorID;references:ID"`
}

func (PlanetPair) TableName() string {
	return "planet_pairs"
}
