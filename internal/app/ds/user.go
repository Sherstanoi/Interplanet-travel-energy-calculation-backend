package ds

type User struct {
	ID       int    `gorm:"column:user_id;primaryKey;autoIncrement"`
	Username string `gorm:"column:username;not null;unique"`
	Password string `gorm:"column:password;not null"`
}

func (User) TableName() string {
	return "users"
}
