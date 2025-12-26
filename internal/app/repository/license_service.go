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
type ServiceInLicenseCalculationRequest struct {
	ID           uint
	Name         string
	Description  string
	ImageURL     string
	BasePrice    float64
	LicenseType  string
	SupportLevel float64 // Индивидуальный коэффициент из LicensePaymentRequestService
	// Расчетные поля (вычисляются на лету из LicensePaymentRequest)
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
func (r *Repository) AddServiceToLicenseCalculationRequest(licenseCalculationRequestID, serviceID uint) error {
	// Проверяем, не добавлена ли уже эта услуга
	var existing ds.LicensePaymentRequestService
	err := r.db.Where("license_calculation_request_id = ? AND service_id = ?", licenseCalculationRequestID, serviceID).First(&existing).Error

	if err == nil {
		// Лицензия уже есть в заявке - просто игнорируем (не ошибка)
		return nil
	}

	// Лицензии нет - добавляем
	licenseCalculationRequestService := ds.LicensePaymentRequestService{
		LicenseCalculationRequestID: licenseCalculationRequestID,
		ServiceID:                   serviceID,
		SupportLevel:                1.0, // По умолчанию
		SubTotal:                    0,   // Будет пересчитано
	}
	err = r.db.Create(&licenseCalculationRequestService).Error
	if err != nil {
		return err
	}

	// Пересчитываем стоимости после добавления услуги
	return r.RecalculateLicenseCalculationRequestCosts(licenseCalculationRequestID)
}

// Удалить услугу из заявки (по licenseCalculationRequestID и serviceID, без ID м-м)
func (r *Repository) RemoveServiceFromLicenseCalculationRequest(licenseCalculationRequestID, serviceID uint) error {
	err := r.db.Where("license_calculation_request_id = ? AND service_id = ?", licenseCalculationRequestID, serviceID).
		Delete(&ds.LicensePaymentRequestService{}).Error
	if err != nil {
		return err
	}

	// Пересчитываем стоимости после удаления услуги
	return r.RecalculateLicenseCalculationRequestCosts(licenseCalculationRequestID)
}

// Создать новую услугу
func (r *Repository) CreateService(name, description, licenseType string, basePrice float64) (*ds.LicenseService, error) {
	service := ds.LicenseService{
		Name:        name,
		Description: description,
		BasePrice:   basePrice,
		LicenseType: licenseType,
		IsDeleted:   false,
	}
	err := r.db.Create(&service).Error
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// Обновить услугу
func (r *Repository) UpdateService(id uint, name, description, licenseType *string, basePrice *float64) error {
	updates := make(map[string]interface{})

	if name != nil && *name != "" {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if basePrice != nil && *basePrice > 0 {
		updates["base_price"] = *basePrice
	}
	if licenseType != nil && *licenseType != "" {
		updates["license_type"] = *licenseType
	}

	if len(updates) == 0 {
		return nil // Нечего обновлять
	}

	return r.db.Model(&ds.LicenseService{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Updates(updates).Error
}

// Логическое удаление услуги
func (r *Repository) DeleteService(id uint) error {
	result := r.db.Model(&ds.LicenseService{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Update("is_deleted", true)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("услуга не найдена или уже удалена")
	}
	return nil
}

// Обновить URL изображения услуги
func (r *Repository) UpdateServiceImage(id uint, imageURL string) error {
	return r.db.Model(&ds.LicenseService{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Update("image_url", imageURL).Error
}

// Удалить изображение услуги (установить в NULL)
func (r *Repository) DeleteServiceImage(id uint) error {
	return r.db.Model(&ds.LicenseService{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Update("image_url", nil).Error
}

// Проверить существует ли услуга
func (r *Repository) ServiceExists(id uint) (bool, error) {
	var count int64
	err := r.db.Model(&ds.LicenseService{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Count(&count).Error
	return count > 0, err
}
