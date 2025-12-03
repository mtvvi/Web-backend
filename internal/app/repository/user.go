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

func (r *Repository) CreateUser(login, password, fullName string, role int) (*ds.User, error) {
	// Если роль не указана или некорректна, устанавливаем Buyer (0)
	if role < 0 || role > 2 {
		role = 0
	}

	user := ds.User{
		Login:       login,
		Password:    password,
		FullName:    fullName,
		Role:        role,
		IsModerator: role == 2, // Совместимость: admin = moderator
	}

	err := r.db.Create(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Получить пользователя по логину
func (r *Repository) GetUserByLogin(login string) (*ds.User, error) {
	var user ds.User
	err := r.db.Where("login = ?", login).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Обновить профиль пользователя
func (r *Repository) UpdateUser(id uint, fullName, password *string) error {
	updates := make(map[string]interface{})

	if fullName != nil && *fullName != "" {
		updates["full_name"] = *fullName
	}
	if password != nil && *password != "" {
		updates["password"] = *password
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.Model(&ds.User{}).Where("id = ?", id).Updates(updates).Error
}

// Проверить существует ли пользователь с таким логином
func (r *Repository) UserExistsByLogin(login string) (bool, error) {
	var count int64
	err := r.db.Model(&ds.User{}).Where("login = ?", login).Count(&count).Error
	return count > 0, err
}
