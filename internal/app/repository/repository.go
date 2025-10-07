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
	PricingType   string  // "per_user", "per_core", "subscription"
}

// Заявка на расчет стоимости лицензирования
type LicenseRequest struct {
	ID                 int     // ID заявки
	CompanyName        string  // Название компании
	UserCount          int     // Количество пользователей
	CoreCount          int     // Общее количество ядер
	LicensePeriodYears int     // Период лицензии (лет)
	SupportLevel       string  // Уровень поддержки: basic, standard, premium
	RequestDate        string  // Дата заявки
	TotalCost          float64 // Итоговая стоимость
	// Связь многие-ко-многим с моделями лицензирования
	LicenseParameters []LicenseToRequest // Поля м-м
}

// Связующая структура для связи многие-ко-многим (Поля м-м)
type LicenseToRequest struct {
	LicenseModelId   int     // ID модели лицензирования
	RequiredQuantity int     // Требуемое количество (вводится пользователем)
	CalculatedCost   float64 // Рассчитанная стоимость (результат расчета)
	AppliedDiscount  float64 // Примененная скидка (результат расчета)
	SupportCoeff     float64 // Коэффициент поддержки (результат расчета)
}

func (r *Repository) GetLicenseModels() ([]LicenseModel, error) {
	models := []LicenseModel{
		{
			ID:            1,
			Name:          "Пользовательские лицензии",
			Description:   "Лицензирование по количеству пользователей. При заказе >100 пользователей скидка 15%. При поддержке 'premium' +30%",
			Icon:          "user.png",
			BasePrice:     25000.0, // руб. за пользователя в год
			SupportCoeff:  1.3,     // Премиум поддержка +30%
			DiscountCoeff: 0.85,    // Скидка при >100 пользователей
			PricingType:   "per_user",
		},
		{
			ID:            2,
			Name:          "Серверные лицензии",
			Description:   "Лицензирование по количеству процессорных ядер. При заказе >50 ядер действует скидка 20%. При поддержке 'standard'/'premium' +25%",
			Icon:          "core.png",
			BasePrice:     150000.0, // руб. за ядро в год
			SupportCoeff:  1.25,     // Техподдержка +25%
			DiscountCoeff: 0.80,     // Скидка при >50 ядер
			PricingType:   "per_core",
		},
		{
			ID:            3,
			Name:          "Корпоративная подписка",
			Description:   "Безлимитная подписка с включенной поддержкой. При заключении контракта >2 лет действует скидка 10%",
			Icon:          "subscription.png",
			BasePrice:     2500000.0, // руб. в год
			SupportCoeff:  1.0,       // Поддержка уже включена
			DiscountCoeff: 0.90,      // Скидка при многолетних контрактах
			PricingType:   "subscription",
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

// Создание новой заявки с дефолтными значениями (как в примере с планетами)
func (r *Repository) GetDefaultLicenseRequest() *LicenseRequest {
	request := &LicenseRequest{
		ID:                 1,
		CompanyName:        "ООО \"Инновационные решения\"",
		UserCount:          50,
		CoreCount:          40,
		LicensePeriodYears: 3,
		SupportLevel:       "standard",
		RequestDate:        "08.10.25",
		TotalCost:          0,
	}

	// Поля м-м (аналог PlanetsParametrs в примере с планетами)
	request.LicenseParameters = []LicenseToRequest{
		{
			LicenseModelId:   1,                 // Пользовательские лицензии
			RequiredQuantity: request.UserCount, // Берется из поля заявки
			CalculatedCost:   0,                 // Результат расчета
			AppliedDiscount:  1.0,               // Результат расчета
			SupportCoeff:     1.0,               // Результат расчета
		},
		{
			LicenseModelId:   2,                 // Серверные лицензии
			RequiredQuantity: request.CoreCount, // Берется из поля заявки
			CalculatedCost:   0,                 // Результат расчета
			AppliedDiscount:  1.0,               // Результат расчета
			SupportCoeff:     1.0,               // Результат расчета
		},
	}

	return request
}

// Расчет стоимости лицензирования с использованием связей многие-ко-многим
func (r *Repository) CalculateLicenseCost(request *LicenseRequest) (*LicenseRequest, error) {
	models, err := r.GetLicenseModels()
	if err != nil {
		return nil, err
	}

	totalCost := 0.0

	// Обрабатываем каждую связь заявка-модель (Поля м-м)
	for i := range request.LicenseParameters {
		licenseParam := &request.LicenseParameters[i]

		// Находим модель лицензирования
		var licenseModel *LicenseModel
		for _, model := range models {
			if model.ID == licenseParam.LicenseModelId {
				licenseModel = &model
				break
			}
		}

		if licenseModel == nil {
			continue
		}

		var modelCost float64

		switch licenseModel.PricingType {
		case "per_user":
			// Базовая стоимость = Требуемое количество пользователей × Цена за пользователя × Годы
			baseCost := float64(licenseParam.RequiredQuantity) * licenseModel.BasePrice * float64(request.LicensePeriodYears)

			// Применяем скидку для больших объемов (>100 пользователей)
			discountCoeff := 1.0
			if licenseParam.RequiredQuantity > 100 {
				discountCoeff = licenseModel.DiscountCoeff
			}

			// Применяем коэффициент поддержки
			supportCoeff := 1.0
			if request.SupportLevel == "premium" {
				supportCoeff = licenseModel.SupportCoeff
			}

			modelCost = baseCost * discountCoeff * supportCoeff

			// Сохраняем рассчитанные коэффициенты
			licenseParam.AppliedDiscount = discountCoeff
			licenseParam.SupportCoeff = supportCoeff

		case "per_core":
			// Базовая стоимость = Требуемое количество ядер × Цена за ядро × Годы
			baseCost := float64(licenseParam.RequiredQuantity) * licenseModel.BasePrice * float64(request.LicensePeriodYears)

			// Скидка для больших серверных инсталляций (>50 ядер)
			discountCoeff := 1.0
			if licenseParam.RequiredQuantity > 50 {
				discountCoeff = licenseModel.DiscountCoeff
			}

			// Применяем коэффициент поддержки
			supportCoeff := 1.0
			if request.SupportLevel != "basic" {
				supportCoeff = licenseModel.SupportCoeff
			}

			modelCost = baseCost * discountCoeff * supportCoeff

			// Сохраняем рассчитанные коэффициенты
			licenseParam.AppliedDiscount = discountCoeff
			licenseParam.SupportCoeff = supportCoeff

		case "subscription":
			// Подписка = Базовая цена × Годы
			baseCost := licenseModel.BasePrice * float64(request.LicensePeriodYears)

			// Скидка при многолетних контрактах (>2 лет)
			discountCoeff := 1.0
			if request.LicensePeriodYears > 2 {
				discountCoeff = licenseModel.DiscountCoeff
			}

			modelCost = baseCost * discountCoeff
			supportCoeff := 1.0 // Поддержка включена в подписку

			// Сохраняем рассчитанные коэффициенты
			licenseParam.AppliedDiscount = discountCoeff
			licenseParam.SupportCoeff = supportCoeff
		}

		// Сохраняем рассчитанную стоимость
		licenseParam.CalculatedCost = modelCost
		totalCost += modelCost
	}

	// Дополнительные расходы (упрощенно)
	implementationCost := totalCost * 0.10
	trainingCost := 0.0
	if request.UserCount > 20 {
		trainingCost = float64(request.UserCount) * 5000.0
	}

	// Итоговая стоимость
	finalCost := totalCost + implementationCost + trainingCost
	request.TotalCost = finalCost

	return request, nil
}

// Получить заявку по ID
func (r *Repository) GetLicenseRequestByID(id int) (*LicenseRequest, error) {
	// В реальном приложении здесь был бы запрос к БД
	// Пока возвращаем дефолтную заявку с расчетами
	request := r.GetDefaultLicenseRequest()
	request.ID = id

	// Выполняем расчет
	calculatedRequest, err := r.CalculateLicenseCost(request)
	if err != nil {
		return nil, err
	}

	return calculatedRequest, nil
}

// Получить лицензии для заявки (аналог GetResearchPlanets)
func (r *Repository) GetRequestLicenses(requestId int) []LicenseModel {
	request, err := r.GetLicenseRequestByID(requestId)
	if err != nil {
		return []LicenseModel{}
	}

	var licensesInRequest []LicenseModel
	for _, licenseParam := range request.LicenseParameters {
		license, err := r.GetLicenseModelByID(licenseParam.LicenseModelId)
		if err == nil {
			licensesInRequest = append(licensesInRequest, license)
		}
	}
	return licensesInRequest
}

// Получить количество лицензий в заявке (аналог GetResearchCount)
func (r *Repository) GetRequestLicenseCount(requestId int) int {
	request, err := r.GetLicenseRequestByID(requestId)
	if err != nil {
		return 0
	}
	return len(request.LicenseParameters)
}

// Получить ID текущей заявки (аналог GetResearchId)
func (r *Repository) GetCurrentRequestId() int {
	return 1
}
