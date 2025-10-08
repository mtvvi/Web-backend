package repository

import (
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &Repository{
		db: db,
	}, nil
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
	LicenseModelId     int     // ID модели лицензирования
	RequiredQuantity   int     // Требуемое количество (вводится пользователем)
	CalculatedCost     float64 // Рассчитанная стоимость (результат расчета)
	BaseCalculatedCost float64 // Цена до применения скидки
	AppliedDiscount    float64 // Примененная скидка (результат расчета)
	SupportCoeff       float64 // Коэффициент поддержки (остается как есть)
}

func (r *Repository) GetLicenseModels() ([]LicenseModel, error) {
	// Получаем данные из БД через новые методы
	services, err := r.GetAllServices()
	if err != nil {
		return []LicenseModel{}, err
	}

	// Преобразуем в старый формат для совместимости с шаблонами
	models := make([]LicenseModel, len(services))
	for i, service := range services {
		// Используем ImageURL из базы или дефолтную картинку если нет
		iconName := "rectangle-2-6.png" // дефолт
		if service.ImageURL != nil && *service.ImageURL != "" {
			iconName = *service.ImageURL
		}

		models[i] = LicenseModel{
			ID:            int(service.ID),
			Name:          service.Name,
			Description:   service.Description,
			Icon:          iconName,
			BasePrice:     service.BasePrice,
			SupportCoeff:  0, // Динамические коэффициенты рассчитываются в калькуляторе
			DiscountCoeff: 0,
			PricingType:   "per_unit",
		}
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

	// Поля м-м
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

func (r *Repository) GetCartCount() int {
	// Используем тестового пользователя ID=1 для демонстрации
	userID := uint(1)

	// Получаем заявку в статусе черновик
	order, err := r.GetOrCreateDraftOrder(userID)
	if err != nil {
		return 0
	}

	// Считаем количество услуг в заявке
	orderServices, err := r.GetOrderServices(order.ID)
	if err != nil {
		return 0
	}

	// Возвращаем общее количество товаров (сумма количества всех услуг)
	totalCount := 0
	for _, service := range orderServices {
		totalCount += service.Quantity
	}

	return totalCount
}
