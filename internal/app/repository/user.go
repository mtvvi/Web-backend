package repository

import (
	"backend/internal/app/ds"
)

// ========= МЕТОДЫ ДЛЯ ПОЛЬЗОВАТЕЛЕЙ (ORM) =========

func (r *Repository) GetUserByID(id uint) (*ds.User, error) {
	var user ds.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateUser(login, password, email, fullName string, isModerator bool) (*ds.User, error) {
	user := ds.User{
		Login:       login,
		Password:    password,
		Email:       email,
		FullName:    fullName,
		IsModerator: isModerator,
	}

	err := r.db.Create(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) GetUserByLogin(login string) (*ds.User, error) {
	var user ds.User
	err := r.db.Where("login = ?", login).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetUserByEmail(email string) (*ds.User, error) {
	var user ds.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetAllUsers() ([]ds.User, error) {
	var users []ds.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *Repository) GetModerators() ([]ds.User, error) {
	var users []ds.User
	err := r.db.Where("is_moderator = ?", true).Find(&users).Error
	return users, err
}

// ========= АДАПТЕРЫ ДЛЯ СОВМЕСТИМОСТИ (будут удалены после миграции) =========

func (r *Repository) GetAllLicenseModelsFromDB() []LicenseModel {
	services, err := r.GetAllServices()
	if err != nil {
		return []LicenseModel{}
	}

	models := make([]LicenseModel, len(services))
	for i, service := range services {
		models[i] = LicenseModel{
			ID:          int(service.ID),
			Name:        service.Name,
			Description: service.Description,
			BasePrice:   service.BasePrice,
			Icon:        "", // Заполним позже при миграции
		}
	}

	return models
}

func (r *Repository) SearchLicenseModelsFromDB(query string) []LicenseModel {
	services, err := r.SearchServicesByName(query)
	if err != nil {
		return []LicenseModel{}
	}

	models := make([]LicenseModel, len(services))
	for i, service := range services {
		models[i] = LicenseModel{
			ID:          int(service.ID),
			Name:        service.Name,
			Description: service.Description,
			BasePrice:   service.BasePrice,
			Icon:        "", // Заполним позже при миграции
		}
	}

	return models
}
