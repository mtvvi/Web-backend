package repository

import (
	"backend/internal/app/ds"
	"database/sql"
	"errors"
)

// Простая структура услуги для отображения
type LicenseService struct {
	ID          uint
	Name        string
	Description string
	ImageURL    string
	BasePrice   float64
	LicenseType string // per_user, per_core, subscription
}

// Структура услуги в заявке (с данными из М-М таблицы)
type ServiceInOrder struct {
	OrderServiceID uint // ID записи в order_services
	ID             uint
	Name           string
	Description    string
	ImageURL       string
	BasePrice      float64
	LicenseType    string
	Users          int     // из таблицы order_services
	Cores          int     // из таблицы order_services
	Period         int     // из таблицы order_services
	SupportLevel   float64 // из таблицы order_services
	SubTotal       float64 // из таблицы order_services
}

// Методы для работы с услугами

// Получить все услуги из БД
func (r *Repository) GetAllServices() ([]LicenseService, error) {
	var dbServices []ds.LicenseService
	err := r.db.Where("is_deleted = ?", false).Find(&dbServices).Error
	if err != nil {
		return nil, err
	}

	services := make([]LicenseService, len(dbServices))
	for i, s := range dbServices {
		imageURL := "rectangle-2-6.png"
		if s.ImageURL != nil && *s.ImageURL != "" {
			imageURL = *s.ImageURL
		}
		services[i] = LicenseService{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			ImageURL:    imageURL,
			BasePrice:   s.BasePrice,
			LicenseType: s.LicenseType,
		}
	}
	return services, nil
}

// Получить услугу по ID
func (r *Repository) GetServiceByID(id uint) (*LicenseService, error) {
	// Используем курсор
	query := `SELECT id, name, description, image_url, base_price, license_type 
	          FROM license_services 
	          WHERE id = $1 AND is_deleted = false`

	// Создание курсора (строковый указатель)
	row := r.db.Raw(query, id).Row()

	// Создание объекта для хранения данных
	var dbID uint
	var name, description, licenseType string
	var imageURLPtr *string
	var basePrice float64

	// Сканирование строки из курсора в переменные
	err := row.Scan(&dbID, &name, &description, &imageURLPtr, &basePrice, &licenseType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Возвращаем nil, если записи нет
		}
		return nil, err
	}

	// Обработка NULL значения для image_url
	imageURL := "rectangle-2-6.png"
	if imageURLPtr != nil && *imageURLPtr != "" {
		imageURL = *imageURLPtr
	}

	service := &LicenseService{
		ID:          dbID,
		Name:        name,
		Description: description,
		ImageURL:    imageURL,
		BasePrice:   basePrice,
		LicenseType: licenseType,
	}
	return service, nil
}

// Поиск услуг по имени
func (r *Repository) SearchServicesByName(name string) ([]LicenseService, error) {
	var dbServices []ds.LicenseService
	err := r.db.Where("name ILIKE ? AND is_deleted = ?", "%"+name+"%", false).Find(&dbServices).Error
	if err != nil {
		return nil, err
	}

	services := make([]LicenseService, len(dbServices))
	for i, s := range dbServices {
		imageURL := "rectangle-2-6.png"
		if s.ImageURL != nil && *s.ImageURL != "" {
			imageURL = *s.ImageURL
		}
		services[i] = LicenseService{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			ImageURL:    imageURL,
			BasePrice:   s.BasePrice,
			LicenseType: s.LicenseType,
		}
	}
	return services, nil
}

// Методы для М-М связей (работа с услугами в заявке)

func (r *Repository) AddServiceToOrder(orderID, serviceID uint, users, cores, period int) error {
	// Получаем цену услуги
	service, err := r.GetServiceByID(serviceID)
	if err != nil {
		return err
	}

	// Уникальный коэффициент поддержки для каждой услуги по ID
	var supportLevel float64
	switch serviceID {
	case 1: // Пользовательские лицензии
		supportLevel = 1.0
	case 2: // Серверные лицензии
		supportLevel = 1.3
	case 3: // Корпоративная подписка
		supportLevel = 1.7
	default:
		// Если нет конкретного ID, используем тип лицензии
		switch service.LicenseType {
		case "per_user":
			supportLevel = 1.0
		case "per_core":
			supportLevel = 1.2
		case "subscription":
			supportLevel = 1.5
		default:
			supportLevel = 1.0
		}
	}

	// Всегда создаем новую запись
	orderService := ds.OrderService{
		OrderID:      orderID,
		ServiceID:    serviceID,
		Users:        users,
		Cores:        cores,
		Period:       period,
		SupportLevel: supportLevel,
		UnitPrice:    service.BasePrice,
		SubTotal:     0, // Будет рассчитано при обновлении параметров
	}

	return r.db.Create(&orderService).Error
}

// Обновить параметры услуги в заявке и пересчитать стоимость
func (r *Repository) UpdateServiceInOrder(orderServiceID uint, users, cores, period int, supportLevel float64) error {
	var os ds.OrderService
	err := r.db.First(&os, orderServiceID).Error
	if err != nil {
		return err
	}

	// Получаем данные услуги
	service, err := r.GetServiceByID(os.ServiceID)
	if err != nil {
		return err
	}

	// Вычисляем количество в зависимости от типа лицензии
	var quantity int
	switch service.LicenseType {
	case "per_user":
		quantity = users
	case "per_core":
		quantity = cores
	case "subscription":
		quantity = period
	default:
		quantity = 1
	}

	// Рассчитываем SubTotal с учетом коэффициента поддержки
	subtotal := service.BasePrice * float64(quantity) * supportLevel

	// Обновляем запись
	os.Users = users
	os.Cores = cores
	os.Period = period
	os.SupportLevel = supportLevel
	os.SubTotal = subtotal

	return r.db.Save(&os).Error
}
