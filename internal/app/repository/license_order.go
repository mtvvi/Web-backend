package repository

import (
	"backend/internal/app/ds"
	"errors"
	"fmt"
	"time"
)

// Методы для работы с заявками

// SQL операция для логического удаления
func (r *Repository) DeleteLicenseCalculationRequest(licenseCalculationRequestID uint) error {
	result := r.db.Exec("UPDATE license_payment_requests SET status = 'удалён' WHERE id = ? AND status = 'черновик'", licenseCalculationRequestID)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("заявку нельзя удалить - неверный статус или ID")
	}

	return nil
}

func (r *Repository) GetLicensePaymentRequestServices(licenseCalculationRequestID uint) ([]ds.LicensePaymentRequestService, error) {
	var licenseCalculationRequestServices []ds.LicensePaymentRequestService
	err := r.db.Where("license_calculation_request_id = ?", licenseCalculationRequestID).Find(&licenseCalculationRequestServices).Error
	return licenseCalculationRequestServices, err
}

// Получить черновик заявки для пользователя (если есть)
func (r *Repository) GetDraftLicenseCalculationRequest(userID uint) (*ds.LicensePaymentRequest, error) {
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("creator_id = ? AND status = ?", userID, "черновик").First(&licenseCalculationRequest).Error
	if err != nil {
		return nil, err
	}
	return &licenseCalculationRequest, nil
}

// Создать новую заявку в статусе черновик
func (r *Repository) CreateDraftLicenseCalculationRequest(userID uint) (*ds.LicensePaymentRequest, error) {
	licenseCalculationRequest := ds.LicensePaymentRequest{
		Status:    "черновик",
		CreatedAt: time.Now(),
		CreatorID: userID,
		Users:     0,
		Cores:     0,
		Period:    0,
	}

	err := r.db.Create(&licenseCalculationRequest).Error
	if err != nil {
		return nil, err
	}

	return &licenseCalculationRequest, nil
}

// Получить заявку по ID (только если она не удалена)
func (r *Repository) GetLicenseCalculationRequestByID(licenseCalculationRequestID uint) (*ds.LicensePaymentRequest, error) {
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("id = ? AND status != ?", licenseCalculationRequestID, "удалён").First(&licenseCalculationRequest).Error
	if err != nil {
		return nil, err
	}
	return &licenseCalculationRequest, nil
}

// Получить заявку или вернуть ошибку
func (r *Repository) GetOrCreateLicenseCalculationRequest(id uint) (*ds.LicensePaymentRequest, error) {
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("id = ? AND status != ?", id, "удалён").First(&licenseCalculationRequest).Error
	if err != nil {
		return nil, fmt.Errorf("заявка не найдена")
	}
	return &licenseCalculationRequest, nil
}

// RecalculateLicenseCalculationRequestCosts пересчитывает стоимость всех услуг в заявке и сохраняет в БД
// ВАЖНО: не рассчитывает sub_total (это делает асинхронный сервис), только суммирует существующие
func (r *Repository) RecalculateLicenseCalculationRequestCosts(licenseCalculationRequestID uint) error {
	// Получаем все связи заявка-услуга
	var licenseCalculationRequestServices []ds.LicensePaymentRequestService
	err := r.db.Where("license_calculation_request_id = ?", licenseCalculationRequestID).Find(&licenseCalculationRequestServices).Error
	if err != nil {
		return err
	}

	if len(licenseCalculationRequestServices) == 0 {
		// Если услуг нет, обнуляем total_cost
		return r.db.Model(&ds.LicensePaymentRequest{}).Where("id = ?", licenseCalculationRequestID).Update("total_cost", 0).Error
	}

	// Суммируем существующие sub_total (расчет происходит асинхронно)
	var totalCost float64
	for _, os := range licenseCalculationRequestServices {
		totalCost += os.SubTotal
	}

	// Обновляем total_cost в заявке
	return r.db.Model(&ds.LicensePaymentRequest{}).Where("id = ?", licenseCalculationRequestID).Update("total_cost", totalCost).Error
}

// Получить услуги в заявке (теперь из БД)
func (r *Repository) GetServicesInLicenseCalculationRequest(licenseCalculationRequestID uint) ([]ServiceInLicenseCalculationRequest, error) {
	// Проверяем что заявка существует и не удалена
	licenseCalculationRequest, err := r.GetLicenseCalculationRequestByID(licenseCalculationRequestID)
	if err != nil {
		return nil, err
	}

	var licenseCalculationRequestServices []ds.LicensePaymentRequestService
	err = r.db.Where("license_calculation_request_id = ?", licenseCalculationRequest.ID).Find(&licenseCalculationRequestServices).Error
	if err != nil {
		return nil, err
	}

	if len(licenseCalculationRequestServices) == 0 {
		return []ServiceInLicenseCalculationRequest{}, nil
	}

	// Получаем ID услуг
	var serviceIDs []uint
	for _, os := range licenseCalculationRequestServices {
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
	services := make([]ServiceInLicenseCalculationRequest, 0, len(licenseCalculationRequestServices))
	for _, os := range licenseCalculationRequestServices {
		s, exists := serviceMap[os.ServiceID]
		if !exists {
			continue // Лицензия удалена
		}

		imageURL := "rectangle-2-6.png"
		if s.ImageURL != nil && *s.ImageURL != "" {
			imageURL = *s.ImageURL
		}

		// Берем subtotal из БД
		services = append(services, ServiceInLicenseCalculationRequest{
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

// Обнулить все sub_total для заявки (перед асинхронным перерасчетом)
func (r *Repository) ResetLicenseCalculationRequestSubTotals(licenseCalculationRequestID uint) error {
	if err := r.db.Model(&ds.LicensePaymentRequestService{}).
		Where("license_calculation_request_id = ?", licenseCalculationRequestID).
		Update("sub_total", 0).Error; err != nil {
		return err
	}

	return r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ?", licenseCalculationRequestID).
		Update("total_cost", 0).Error
}

// Установить sub_total для услуги и пересчитать total_cost как сумму sub_total
func (r *Repository) UpdateLicenseCalculationRequestSubTotal(licenseCalculationRequestID, serviceID uint, subTotal float64) error {
	result := r.db.Model(&ds.LicensePaymentRequestService{}).
		Where("license_calculation_request_id = ? AND service_id = ?", licenseCalculationRequestID, serviceID).
		Update("sub_total", subTotal)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("услуга в заявке не найдена")
	}

	// Считаем сумму
	var sum float64
	err := r.db.Model(&ds.LicensePaymentRequestService{}).
		Where("license_calculation_request_id = ?", licenseCalculationRequestID).
		Select("COALESCE(SUM(sub_total), 0)").Scan(&sum).Error
	if err != nil {
		return err
	}

	return r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ?", licenseCalculationRequestID).
		Update("total_cost", sum).Error
}

// Получить количество услуг в заявке (количество записей, не сумму)
func (r *Repository) GetLicenseCalculationRequestCount(licenseCalculationRequestID uint) int {
	licenseCalculationRequest, err := r.GetLicenseCalculationRequestByID(licenseCalculationRequestID)
	if err != nil {
		return 0
	}

	var count int64
	err = r.db.Model(&ds.LicensePaymentRequestService{}).Where("license_calculation_request_id = ?", licenseCalculationRequest.ID).Count(&count).Error
	if err != nil {
		return 0
	}

	return int(count)
}

// Количество услуг в заявке, для которых рассчитан sub_total (>0)
func (r *Repository) CountCalculatedServices(licenseCalculationRequestID uint) int {
	licenseCalculationRequest, err := r.GetLicenseCalculationRequestByID(licenseCalculationRequestID)
	if err != nil {
		return 0
	}

	var count int64
	err = r.db.Model(&ds.LicensePaymentRequestService{}).
		Where("license_calculation_request_id = ? AND sub_total > 0", licenseCalculationRequest.ID).
		Count(&count).Error
	if err != nil {
		return 0
	}

	return int(count)
}

// Получить количество в корзине (черновик для пользователя)
func (r *Repository) GetCartCount() int {
	userID := uint(1)
	licenseCalculationRequest, err := r.GetDraftLicenseCalculationRequest(userID)
	if err != nil {
		return 0 // Нет черновика - корзина пуста
	}

	var count int64
	err = r.db.Model(&ds.LicensePaymentRequestService{}).Where("license_calculation_request_id = ?", licenseCalculationRequest.ID).Count(&count).Error
	if err != nil {
		return 0
	}

	return int(count)
}

// Получить ID черновика заявки (или 0 если нет)
func (r *Repository) GetDraftLicenseCalculationRequestID(userID uint) uint {
	licenseCalculationRequest, err := r.GetDraftLicenseCalculationRequest(userID)
	if err != nil {
		return 0
	}
	return licenseCalculationRequest.ID
}

// Обновить параметры расчета в заявке (теперь без supportLevel)
func (r *Repository) UpdateLicenseCalculationRequestParams(licenseCalculationRequestID uint, users, cores, period int) error {
	return r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ?", licenseCalculationRequestID).
		Updates(map[string]interface{}{
			"users":  users,
			"cores":  cores,
			"period": period,
		}).Error
}

// Обновить коэффициент поддержки для конкретной услуги в заявке
func (r *Repository) UpdateServiceSupportLevel(licenseCalculationRequestID, serviceID uint, supportLevel float64) error {
	// Ограничиваем значение в диапазоне 0.7 - 3.0
	if supportLevel < 0.7 {
		supportLevel = 0.7
	}
	if supportLevel > 3.0 {
		supportLevel = 3.0
	}

	err := r.db.Model(&ds.LicensePaymentRequestService{}).
		Where("license_calculation_request_id = ? AND service_id = ?", licenseCalculationRequestID, serviceID).
		Update("support_level", supportLevel).Error
	if err != nil {
		return err
	}

	// Пересчитываем стоимость после изменения коэффициента
	return r.RecalculateLicenseCalculationRequestCosts(licenseCalculationRequestID)
}

// Получить все заявки с фильтрацией (кроме удаленных и черновиков)
func (r *Repository) GetLicenseCalculationRequests(status string, dateFrom, dateTo *time.Time, creatorID *uint) ([]ds.LicensePaymentRequest, error) {
	query := r.db.Model(&ds.LicensePaymentRequest{})

	// Фильтрация по создателю (для обычных пользователей)
	if creatorID != nil {
		query = query.Where("creator_id = ?", *creatorID)
	}

	if status != "" && status != "все" {
		query = query.Where("status = ?", status)
	}

	if dateFrom != nil {
		// Сравниваем только дату, игнорируя время
		query = query.Where("DATE(formatted_at) >= DATE(?)", dateFrom)
	}

	if dateTo != nil {
		// Сравниваем только дату, игнорируя время
		query = query.Where("DATE(formatted_at) <= DATE(?)", dateTo)
	}

	var licenseCalculationRequests []ds.LicensePaymentRequest
	err := query.Preload("Creator").Preload("Moderator").Order("created_at DESC").Find(&licenseCalculationRequests).Error
	return licenseCalculationRequests, err
}

// Обновить поля заявки (только допустимые для изменения)
func (r *Repository) UpdateLicenseCalculationRequestFields(licenseCalculationRequestID uint, users, cores, period *int) error {
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

	err := r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ? AND status = ?", licenseCalculationRequestID, "черновик").
		Updates(updates).Error
	if err != nil {
		return err
	}

	// Пересчитываем стоимость после изменения параметров
	return r.RecalculateLicenseCalculationRequestCosts(licenseCalculationRequestID)
}

// Сформировать заявку (создателем)
func (r *Repository) FormatLicenseCalculationRequest(licenseCalculationRequestID uint) error {
	// Проверяем обязательные поля
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("id = ? AND status = ?", licenseCalculationRequestID, "черновик").First(&licenseCalculationRequest).Error
	if err != nil {
		return errors.New("заявка не найдена или не в статусе черновик")
	}

	// Проверяем наличие услуг в заявке
	var count int64
	r.db.Model(&ds.LicensePaymentRequestService{}).Where("license_calculation_request_id = ?", licenseCalculationRequestID).Count(&count)
	if count == 0 {
		return errors.New("нельзя сформировать пустую заявку")
	}

	// Проверяем обязательные поля
	if licenseCalculationRequest.Users <= 0 && licenseCalculationRequest.Cores <= 0 {
		return errors.New("необходимо указать количество пользователей или ядер")
	}
	if licenseCalculationRequest.Period <= 0 {
		return errors.New("необходимо указать период лицензирования")
	}

	// Обнуляем sub_total для всех услуг (расчет будет в асинхронном сервисе после завершения)
	err = r.ResetLicenseCalculationRequestSubTotals(licenseCalculationRequestID)
	if err != nil {
		return err
	}

	now := time.Now()
	return r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ?", licenseCalculationRequestID).
		Updates(map[string]interface{}{
			"status":       "сформирован",
			"formatted_at": now,
		}).Error
}

// Завершить заявку (модератором) с расчетом стоимости
func (r *Repository) CompleteLicenseCalculationRequest(licenseCalculationRequestID, moderatorID uint) error {
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("id = ? AND status = ?", licenseCalculationRequestID, "сформирован").First(&licenseCalculationRequest).Error
	if err != nil {
		return errors.New("заявка не найдена или не в статусе сформирован")
	}

	now := time.Now()
	return r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ?", licenseCalculationRequestID).
		Updates(map[string]interface{}{
			"status":       "завершён",
			"completed_at": now,
			"moderator_id": moderatorID,
		}).Error
}

// Отклонить заявку (модератором)
func (r *Repository) RejectLicenseCalculationRequest(licenseCalculationRequestID, moderatorID uint) error {
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("id = ? AND status = ?", licenseCalculationRequestID, "сформирован").First(&licenseCalculationRequest).Error
	if err != nil {
		return errors.New("заявка не найдена или не в статусе сформирован")
	}

	now := time.Now()
	return r.db.Model(&ds.LicensePaymentRequest{}).
		Where("id = ?", licenseCalculationRequestID).
		Updates(map[string]interface{}{
			"status":       "отклонён",
			"completed_at": now,
			"moderator_id": moderatorID,
		}).Error
}

// Получить заявку с услугами и расчетом стоимости
func (r *Repository) GetLicenseCalculationRequestWithServices(licenseCalculationRequestID uint) (*ds.LicensePaymentRequest, []ServiceInLicenseCalculationRequest, float64, error) {
	var licenseCalculationRequest ds.LicensePaymentRequest
	err := r.db.Where("id = ? AND status != ?", licenseCalculationRequestID, "удалён").
		Preload("Creator").
		Preload("Moderator").
		First(&licenseCalculationRequest).Error
	if err != nil {
		return nil, nil, 0, err
	}

	services, err := r.GetServicesInLicenseCalculationRequest(licenseCalculationRequestID)
	if err != nil {
		return nil, nil, 0, err
	}

	// Возвращаем TotalCost из БД
	return &licenseCalculationRequest, services, licenseCalculationRequest.TotalCost, nil
}
