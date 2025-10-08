package repository

import (
	"backend/internal/app/ds"
)

// ========= МЕТОДЫ ДЛЯ УСЛУГ (ORM) =========

func (r *Repository) GetAllServices() ([]ds.LicenseService, error) {
	var services []ds.LicenseService
	err := r.db.Where("is_deleted = ?", false).Find(&services).Error
	if err != nil {
		return nil, err
	}
	return services, nil
}

func (r *Repository) GetServiceByID(id uint) (*ds.LicenseService, error) {
	var service ds.LicenseService
	err := r.db.Where("id = ? AND is_deleted = ?", id, false).First(&service).Error
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func (r *Repository) SearchServicesByName(name string) ([]ds.LicenseService, error) {
	var services []ds.LicenseService
	err := r.db.Where("name ILIKE ? AND is_deleted = ?", "%"+name+"%", false).Find(&services).Error
	if err != nil {
		return nil, err
	}
	return services, nil
}

//  МЕТОДЫ ДЛЯ М-М СВЯЗЕЙ (ORM)

func (r *Repository) AddServiceToOrder(orderID, serviceID uint, quantity int) error {

	var existing ds.OrderService
	err := r.db.Where("order_id = ? AND service_id = ?", orderID, serviceID).First(&existing).Error

	if err == nil {

		existing.Quantity += quantity
		return r.db.Save(&existing).Error
	}

	// Получаем цену услуги
	service, err := r.GetServiceByID(serviceID)
	if err != nil {
		return err
	}

	// Создаем новую связь
	orderService := ds.OrderService{
		OrderID:   orderID,
		ServiceID: serviceID,
		Quantity:  quantity,
		UnitPrice: service.BasePrice,
		SubTotal:  service.BasePrice * float64(quantity),
	}

	return r.db.Create(&orderService).Error
}
