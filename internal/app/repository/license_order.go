package repository

import (
	"backend/internal/app/ds"
	"errors"
	"fmt"
	"time"
)

// Методы для работы с заявками

// SQL операция для логического удаления
func (r *Repository) DeleteOrder(orderID uint) error {
	result := r.db.Exec("UPDATE license_orders SET status = 'удалён' WHERE id = ? AND status = 'черновик'", orderID)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("заявку нельзя удалить - неверный статус или ID")
	}

	return nil
}

func (r *Repository) GetOrderServices(orderID uint) ([]ds.OrderService, error) {
	var orderServices []ds.OrderService
	err := r.db.Where("order_id = ?", orderID).Find(&orderServices).Error
	return orderServices, err
}

// Получить черновик заявки для пользователя (если есть)
func (r *Repository) GetDraftOrder(userID uint) (*ds.LicenseOrder, error) {
	var order ds.LicenseOrder
	err := r.db.Where("creator_id = ? AND status = ?", userID, "черновик").First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Создать новую заявку в статусе черновик
func (r *Repository) CreateDraftOrder(userID uint) (*ds.LicenseOrder, error) {
	order := ds.LicenseOrder{
		Status:    "черновик",
		CreatedAt: time.Now(),
		CreatorID: userID,
		Users:     0,
		Cores:     0,
		Period:    0,
	}

	err := r.db.Create(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// Получить заявку по ID (только если она не удалена)
func (r *Repository) GetOrderByID(orderID uint) (*ds.LicenseOrder, error) {
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status != ?", orderID, "удалён").First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Получить заявку или вернуть ошибку
func (r *Repository) GetOrCreateOrder(id uint) (*ds.LicenseOrder, error) {
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status != ?", id, "удалён").First(&order).Error
	if err != nil {
		return nil, fmt.Errorf("заявка не найдена")
	}
	return &order, nil
}

// RecalculateOrderCosts пересчитывает стоимость всех услуг в заявке и сохраняет в БД
func (r *Repository) RecalculateOrderCosts(orderID uint) error {
	// Получаем заявку
	order, err := r.GetOrderByID(orderID)
	if err != nil {
		return err
	}

	// Получаем все связи заявка-услуга
	var orderServices []ds.OrderService
	err = r.db.Where("order_id = ?", orderID).Find(&orderServices).Error
	if err != nil {
		return err
	}

	if len(orderServices) == 0 {
		// Если услуг нет, обнуляем total_cost
		return r.db.Model(&ds.LicenseOrder{}).Where("id = ?", orderID).Update("total_cost", 0).Error
	}

	// Получаем все услуги
	var serviceIDs []uint
	for _, os := range orderServices {
		serviceIDs = append(serviceIDs, os.ServiceID)
	}

	var dbServices []ds.LicenseService
	err = r.db.Where("id IN ? AND is_deleted = ?", serviceIDs, false).Find(&dbServices).Error
	if err != nil {
		return err
	}

	// Создаем map для быстрого доступа
	serviceMap := make(map[uint]ds.LicenseService)
	for _, s := range dbServices {
		serviceMap[s.ID] = s
	}

	// Пересчитываем стоимость для каждой услуги и обновляем в БД
	var totalCost float64
	for _, os := range orderServices {
		s, exists := serviceMap[os.ServiceID]
		if !exists {
			continue // Услуга удалена
		}

		// Вычисляем количество в зависимости от типа лицензии
		var quantity int
		switch s.LicenseType {
		case "per_user":
			quantity = order.Users
		case "per_core":
			quantity = order.Cores
		case "subscription":
			quantity = order.Period
		default:
			quantity = 1
		}

		// Рассчитываем стоимость
		subtotal := s.BasePrice * float64(quantity) * os.SupportLevel

		// Обновляем subtotal в БД
		err = r.db.Model(&ds.OrderService{}).
			Where("order_id = ? AND service_id = ?", orderID, os.ServiceID).
			Update("sub_total", subtotal).Error
		if err != nil {
			return err
		}

		totalCost += subtotal
	}

	// Обновляем total_cost в заявке
	return r.db.Model(&ds.LicenseOrder{}).Where("id = ?", orderID).Update("total_cost", totalCost).Error
}

// Получить услуги в заявке (теперь из БД)
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

	// Получаем ID услуг
	var serviceIDs []uint
	for _, os := range orderServices {
		serviceIDs = append(serviceIDs, os.ServiceID)
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

	// Создаем список услуг с расчетом стоимости ИЗ БД
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

		// Берем subtotal из БД
		services = append(services, ServiceInOrder{
			ID:           s.ID,
			Name:         s.Name,
			Description:  s.Description,
			ImageURL:     imageURL,
			BasePrice:    s.BasePrice,
			LicenseType:  s.LicenseType,
			SupportLevel: os.SupportLevel,
			SubTotal:     os.SubTotal, // Из БД!
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

// Обновить параметры расчета в заявке (теперь без supportLevel)
func (r *Repository) UpdateOrderParams(orderID uint, users, cores, period int) error {
	return r.db.Model(&ds.LicenseOrder{}).
		Where("id = ?", orderID).
		Updates(map[string]interface{}{
			"users":  users,
			"cores":  cores,
			"period": period,
		}).Error
}

// Обновить коэффициент поддержки для конкретной услуги в заявке
func (r *Repository) UpdateServiceSupportLevel(orderID, serviceID uint, supportLevel float64) error {
	// Ограничиваем значение в диапазоне 0.7 - 3.0
	if supportLevel < 0.7 {
		supportLevel = 0.7
	}
	if supportLevel > 3.0 {
		supportLevel = 3.0
	}

	err := r.db.Model(&ds.OrderService{}).
		Where("order_id = ? AND service_id = ?", orderID, serviceID).
		Update("support_level", supportLevel).Error
	if err != nil {
		return err
	}

	// Пересчитываем стоимость после изменения коэффициента
	return r.RecalculateOrderCosts(orderID)
}

// Получить все заявки с фильтрацией (кроме удаленных и черновиков)
func (r *Repository) GetOrders(status string, dateFrom, dateTo *time.Time, creatorID *uint) ([]ds.LicenseOrder, error) {
	query := r.db.Model(&ds.LicenseOrder{})

	// Фильтрация по создателю (для обычных пользователей)
	if creatorID != nil {
		query = query.Where("creator_id = ?", *creatorID)
	}

	if status != "" && status != "все" {
		query = query.Where("status = ?", status)
	}

	if dateFrom != nil {
		query = query.Where("formatted_at >= ?", dateFrom)
	}

	if dateTo != nil {
		query = query.Where("formatted_at <= ?", dateTo)
	}

	var orders []ds.LicenseOrder
	err := query.Preload("Creator").Preload("Moderator").Order("created_at DESC").Find(&orders).Error
	return orders, err
}

// Обновить поля заявки (только допустимые для изменения)
func (r *Repository) UpdateOrderFields(orderID uint, users, cores, period *int) error {
	updates := make(map[string]interface{})

	if users != nil {
		updates["users"] = *users
	}
	if cores != nil {
		updates["cores"] = *cores
	}
	if period != nil {
		updates["period"] = *period
	}

	if len(updates) == 0 {
		return nil
	}

	err := r.db.Model(&ds.LicenseOrder{}).
		Where("id = ? AND status = ?", orderID, "черновик").
		Updates(updates).Error
	if err != nil {
		return err
	}

	// Пересчитываем стоимость после изменения параметров
	return r.RecalculateOrderCosts(orderID)
}

// Сформировать заявку (создателем)
func (r *Repository) FormatOrder(orderID uint) error {
	// Проверяем обязательные поля
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status = ?", orderID, "черновик").First(&order).Error
	if err != nil {
		return errors.New("заявка не найдена или не в статусе черновик")
	}

	// Проверяем наличие услуг в заявке
	var count int64
	r.db.Model(&ds.OrderService{}).Where("order_id = ?", orderID).Count(&count)
	if count == 0 {
		return errors.New("нельзя сформировать пустую заявку")
	}

	// Проверяем обязательные поля
	if order.Users <= 0 && order.Cores <= 0 {
		return errors.New("необходимо указать количество пользователей или ядер")
	}
	if order.Period <= 0 {
		return errors.New("необходимо указать период лицензирования")
	}

	// Пересчитываем стоимость перед формированием
	err = r.RecalculateOrderCosts(orderID)
	if err != nil {
		return err
	}

	now := time.Now()
	return r.db.Model(&ds.LicenseOrder{}).
		Where("id = ?", orderID).
		Updates(map[string]interface{}{
			"status":       "сформирован",
			"formatted_at": now,
		}).Error
}

// Завершить заявку (модератором) с расчетом стоимости
func (r *Repository) CompleteOrder(orderID, moderatorID uint) error {
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status = ?", orderID, "сформирован").First(&order).Error
	if err != nil {
		return errors.New("заявка не найдена или не в статусе сформирован")
	}

	now := time.Now()
	return r.db.Model(&ds.LicenseOrder{}).
		Where("id = ?", orderID).
		Updates(map[string]interface{}{
			"status":       "завершён",
			"completed_at": now,
			"moderator_id": moderatorID,
		}).Error
}

// Отклонить заявку (модератором)
func (r *Repository) RejectOrder(orderID, moderatorID uint) error {
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status = ?", orderID, "сформирован").First(&order).Error
	if err != nil {
		return errors.New("заявка не найдена или не в статусе сформирован")
	}

	now := time.Now()
	return r.db.Model(&ds.LicenseOrder{}).
		Where("id = ?", orderID).
		Updates(map[string]interface{}{
			"status":       "отклонён",
			"completed_at": now,
			"moderator_id": moderatorID,
		}).Error
}

// Получить заявку с услугами и расчетом стоимости
func (r *Repository) GetOrderWithServices(orderID uint) (*ds.LicenseOrder, []ServiceInOrder, float64, error) {
	var order ds.LicenseOrder
	err := r.db.Where("id = ? AND status != ?", orderID, "удалён").
		Preload("Creator").
		Preload("Moderator").
		First(&order).Error
	if err != nil {
		return nil, nil, 0, err
	}

	services, err := r.GetServicesInOrder(orderID)
	if err != nil {
		return nil, nil, 0, err
	}

	// Возвращаем TotalCost из БД
	return &order, services, order.TotalCost, nil
}
