package repository

import (
	"backend/internal/app/ds"
)

// Методы для пользователей (ORM)

func (r *Repository) GetUserByID(id uint) (*ds.User, error) {
	var user ds.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateUser(login, password, fullName string, isModerator bool) (*ds.User, error) {
	user := ds.User{
		Login:       login,
		Password:    password,
		FullName:    fullName,
		IsModerator: isModerator,
	}

	err := r.db.Create(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}
