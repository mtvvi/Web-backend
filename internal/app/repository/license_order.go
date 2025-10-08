package repository

import (
	"backend/internal/app/ds"
	"errors"
	"time"
)

// ========= ОСНОВНЫЕ МЕТОДЫ ЗАЯВОК =========

// SQL операция для логического удаления (как требует задание)
func (r *Repository) DeleteOrder(orderID uint) error {
	// Используем SQL напрямую для демонстрации
	result := r.db.Exec("UPDATE license_orders SET status = 'удалён' WHERE id = ? AND status = 'черновик'", orderID)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("заявку нельзя удалить - неверный статус или ID")
	}

	return nil
}

// ========= МЕТОДЫ УПРАВЛЕНИЯ КОРЗИНОЙ =========

func (r *Repository) GetOrCreateDraftOrder(userID uint) (*ds.LicenseOrder, error) {
	var order ds.LicenseOrder

	// Ищем существующий черновик
	err := r.db.Where("creator_id = ? AND status = ?", userID, "черновик").First(&order).Error
	if err == nil {
		return &order, nil
	}

	// Создаем новый черновик если не найден
	order = ds.LicenseOrder{
		Status:        "черновик",
		CreatedAt:     time.Now(),
		CreatorID:     userID,
		CompanyName:   "Тестовая компания",
		LicensePeriod: 1,
		ContactEmail:  "test@example.com",
		Priority:      "medium",
	}

	err = r.db.Create(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *Repository) GetOrderServices(orderID uint) ([]ds.OrderService, error) {
	var orderServices []ds.OrderService
	err := r.db.Where("order_id = ?", orderID).Find(&orderServices).Error
	return orderServices, err
}
