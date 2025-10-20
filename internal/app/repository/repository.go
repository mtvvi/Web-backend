package repository

import (
	"backend/internal/app/ds"
	"fmt"

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

	// Автоматическая миграция всех таблиц
	err = db.AutoMigrate(
		&ds.User{},
		&ds.LicenseService{},
		&ds.LicenseOrder{},
		&ds.OrderService{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Repository{
		db: db,
	}, nil
}

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
	var dbService ds.LicenseService
	err := r.db.Where("id = ? AND is_deleted = ?", id, false).First(&dbService).Error
	if err != nil {
		return nil, err
	}

	imageURL := "rectangle-2-6.png"
	if dbService.ImageURL != nil && *dbService.ImageURL != "" {
		imageURL = *dbService.ImageURL
	}

	service := &LicenseService{
		ID:          dbService.ID,
		Name:        dbService.Name,
		Description: dbService.Description,
		ImageURL:    imageURL,
		BasePrice:   dbService.BasePrice,
		LicenseType: dbService.LicenseType,
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

// Получить заявку или создать новую
func (r *Repository) GetOrCreateOrder(id uint) (*ds.LicenseOrder, error) {
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status != ?", id, "удалён").First(&order).Error
	if err != nil {
		return nil, fmt.Errorf("заявка не найдена")
	}
	return &order, nil
}

// Получить услуги в заявке (с данными из М-М таблицы)
func (r *Repository) GetServicesInOrder(orderID uint) ([]ServiceInOrder, error) {
	// Проверяем что заявка существует и не удалена
	order, err := r.GetOrderByID(orderID)
	if err != nil {
		return nil, err
	}

	var orderServices []ds.OrderService
	err = r.db.Where("order_id = ?", order.ID).Find(&orderServices).Error
	if err != nil {
		return nil, err
	}

	if len(orderServices) == 0 {
		return []ServiceInOrder{}, nil
	}

	// Получаем уникальные ID услуг
	serviceIDMap := make(map[uint]bool)
	for _, os := range orderServices {
		serviceIDMap[os.ServiceID] = true
	}

	var serviceIDs []uint
	for id := range serviceIDMap {
		serviceIDs = append(serviceIDs, id)
	}

	var dbServices []ds.LicenseService
	err = r.db.Where("id IN ? AND is_deleted = ?", serviceIDs, false).Find(&dbServices).Error
	if err != nil {
		return nil, err
	}

	// Создаем map для быстрого доступа к данным услуг
	serviceMap := make(map[uint]ds.LicenseService)
	for _, s := range dbServices {
		serviceMap[s.ID] = s
	}

	// Создаем список услуг в заявке (каждая запись М-М = отдельный элемент)
	services := make([]ServiceInOrder, 0, len(orderServices))
	for _, os := range orderServices {
		s, exists := serviceMap[os.ServiceID]
		if !exists {
			continue // Услуга удалена
		}

		imageURL := "rectangle-2-6.png"
		if s.ImageURL != nil && *s.ImageURL != "" {
			imageURL = *s.ImageURL
		}

		services = append(services, ServiceInOrder{
			OrderServiceID: os.ID,
			ID:             s.ID,
			Name:           s.Name,
			Description:    s.Description,
			ImageURL:       imageURL,
			BasePrice:      s.BasePrice,
			LicenseType:    s.LicenseType,
			Users:          os.Users,
			Cores:          os.Cores,
			Period:         os.Period,
			SupportLevel:   os.SupportLevel,
			SubTotal:       os.SubTotal,
		})
	}
	return services, nil
}

// Получить количество услуг в заявке (количество записей, не сумму)
func (r *Repository) GetOrderCount(orderID uint) int {
	order, err := r.GetOrderByID(orderID)
	if err != nil {
		return 0
	}

	var count int64
	err = r.db.Model(&ds.OrderService{}).Where("order_id = ?", order.ID).Count(&count).Error
	if err != nil {
		return 0
	}

	return int(count)
}

// Получить количество в корзине (черновик для пользователя)
func (r *Repository) GetCartCount() int {
	userID := uint(1)
	order, err := r.GetDraftOrder(userID)
	if err != nil {
		return 0 // Нет черновика - корзина пуста
	}

	var count int64
	err = r.db.Model(&ds.OrderService{}).Where("order_id = ?", order.ID).Count(&count).Error
	if err != nil {
		return 0
	}

	return int(count)
}

// Получить ID черновика заявки (или 0 если нет)
func (r *Repository) GetDraftOrderID(userID uint) uint {
	order, err := r.GetDraftOrder(userID)
	if err != nil {
		return 0
	}
	return order.ID
}
