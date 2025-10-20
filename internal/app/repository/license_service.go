package repository

import (
	"backend/internal/app/ds"
)

// Методы для М-М связей (ORM)

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

	// Всегда создаем новую запись (не проверяем дубликаты)
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
