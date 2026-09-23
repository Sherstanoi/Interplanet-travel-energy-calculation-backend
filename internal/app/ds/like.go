package ds

type Like struct {
	ID           int `gorm:"column:like_id;primaryKey"`
	PlanetPairID int `gorm:"column:planetpair_id;not null;uniqueIndex:uq_likes_user_planetpair"`
	UserID       int `gorm:"column:user_id;not null;uniqueIndex:uq_likes_user_planetpair"`

	PlanetPair PlanetPair `gorm:"foreignKey:PlanetPairID;references:ID"`
	User       User       `gorm:"foreignKey:UserID;references:ID"`
}

func (Like) TableName() string {
	return "likes"
}