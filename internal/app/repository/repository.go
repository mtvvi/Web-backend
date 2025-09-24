package repository

import (
	"fmt"
	"strings"
)

type Repository struct {
}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type LicenseModel struct {
	ID            int
	Name          string
	Description   string
	Icon          string
	BasePrice     float64 // Цена за единицу
	SupportCoeff  float64 // Коэффициент поддержки
	DiscountCoeff float64 // Коэффициент скидки
}

func (r *Repository) GetLicenseModels() ([]LicenseModel, error) {
	models := []LicenseModel{
		{
			ID:            1,
			Name:          "На пользователя",
			Description:   "Модель лицензирования per user",
			Icon:          "user.png",
			BasePrice:     100.0,
			SupportCoeff:  1.2,
			DiscountCoeff: 0.85,
		},
		{
			ID:            2,
			Name:          "На ядро",
			Description:   "Модель лицензирования per core",
			Icon:          "core.png",
			BasePrice:     50.0,
			SupportCoeff:  1.15,
			DiscountCoeff: 0.9,
		},
		{
			ID:            3,
			Name:          "Подписка",
			Description:   "Подписочная модель лицензирование subscription",
			Icon:          "subscription.png",
			BasePrice:     10.0,
			SupportCoeff:  1.1,
			DiscountCoeff: 0.95,
		},
	}

	return models, nil
}

func (r *Repository) GetLicenseModelsByName(name string) ([]LicenseModel, error) {
	models, err := r.GetLicenseModels()
	if err != nil {
		return []LicenseModel{}, err
	}

	var result []LicenseModel
	for _, model := range models {
		// Поиск только по названию модели
		if strings.Contains(strings.ToLower(model.Name), strings.ToLower(name)) {
			result = append(result, model)
		}
	}

	return result, nil
}

func (r *Repository) GetCartModels() ([]LicenseModel, error) {
	// Возвращаем только модели, которые в заявке (корзине) - без подписки
	models, err := r.GetLicenseModels()
	if err != nil {
		return []LicenseModel{}, err
	}

	// Возвращаем только первые 2 модели (исключаем подписку)
	cartModels := make([]LicenseModel, 0)
	for _, model := range models {
		if model.Name != "Подписка" {
			cartModels = append(cartModels, model)
		}
	}

	return cartModels, nil
}

func (r *Repository) GetCartCount() int {
	// Возвращаем количество товаров в заявке
	return 2
}

func (r *Repository) GetLicenseModelByID(id int) (LicenseModel, error) {
	// Получаем все лицензионные модели
	models, err := r.GetLicenseModels()
	if err != nil {
		return LicenseModel{}, err
	}

	// Ищем модель по ID
	for _, model := range models {
		if model.ID == id {
			return model, nil
		}
	}

	// Если модель не найдена, возвращаем ошибку
	return LicenseModel{}, fmt.Errorf("license model with ID %d not found", id)
}
