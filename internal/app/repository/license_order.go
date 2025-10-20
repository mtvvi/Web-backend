package repository

import (
	"backend/internal/app/ds"
	"errors"
	"time"

	"gorm.io/gorm"
)

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
		// Возвращаем nil без логирования если просто не найдено
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &order, nil
}

// Создать новую заявку в статусе черновик
func (r *Repository) CreateDraftOrder(userID uint) (*ds.LicenseOrder, error) {
	order := ds.LicenseOrder{
		Status:        "черновик",
		CreatedAt:     time.Now(),
		CreatorID:     userID,
		CompanyName:   "",
		LicensePeriod: 1,
		ContactEmail:  "",
		Priority:      "medium",
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
