package auth

import "sync"

type currentUser struct {
	id int
}

var (
	instance *currentUser
	once     sync.Once
)

func CurrentUser() *currentUser {
	once.Do(func() {
		instance = &currentUser{id: 1} 
	})
	return instance
}

func (u *currentUser) ID() int { return u.id }

func CurrentUserID() int { return CurrentUser().ID() }
