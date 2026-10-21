package repository

import (
	"errors"

	"lab1/internal/app/ds"

	"gorm.io/gorm"
)

// GetUserByUsername ищет пользователя по логину (нужен для аутентификации).
// Если пользователя нет, возвращает ErrNotFound.
func (r *Repository) GetUserByUsername(username string) (*ds.User, error) {
	var u ds.User
	err := r.db.Where("username = ?", username).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
