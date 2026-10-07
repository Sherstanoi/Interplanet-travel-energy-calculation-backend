package main

import (
	"lab1/internal/app/ds"
	"lab1/internal/app/dsn"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	err = db.AutoMigrate(
		&ds.User{},
		&ds.PlanetPair{},
		&ds.Like{},
	)
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_one_draft_per_user
		ON planet_pairs (creator_id) WHERE status = 'draft'`).Error
	if err != nil {
		panic("cant create draft index")
	}
	if err != nil {
		panic("cant migrate db")
	}
}
