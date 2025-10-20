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

// Структура услуги в заявке (данные из license_services + расчет на лету)
type ServiceInOrder struct {
	ID           uint
	Name         string
	Description  string
	ImageURL     string
	BasePrice    float64
	LicenseType  string
	SupportLevel float64 // Индивидуальный коэффициент из OrderService
	// Расчетные поля (вычисляются на лету из LicenseOrder)
	SubTotal float64 // BasePrice × quantity × SupportLevel
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

// Методы для М-М связей (простая связь услуг с заявкой)

// Добавить услугу в заявку (с дефолтным коэффициентом поддержки)
func (r *Repository) AddServiceToOrder(orderID, serviceID uint) error {
	// Проверяем, не добавлена ли уже эта услуга
	var existing ds.OrderService
	err := r.db.Where("order_id = ? AND service_id = ?", orderID, serviceID).First(&existing).Error

	if err == nil {
		// Услуга уже есть в заявке - просто игнорируем (не ошибка)
		return nil
	}

	// Услуги нет - добавляем
	orderService := ds.OrderService{
		OrderID:      orderID,
		ServiceID:    serviceID,
		SupportLevel: 1.0, // По умолчанию
	}
	return r.db.Create(&orderService).Error
}

// Удалить услугу из заявки
func (r *Repository) RemoveServiceFromOrder(orderServiceID uint) error {
	return r.db.Delete(&ds.OrderService{}, orderServiceID).Error
}
