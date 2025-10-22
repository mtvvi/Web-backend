package handler

import (
	"backend/internal/app/dto"
	"backend/internal/app/repository"
	"backend/internal/app/storage"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Простые функции для получения текущего пользователя (singleton)
func getCurrentCreatorID() uint {
	return 1
}

func getCurrentModeratorID() uint {
	return 2
}

// APIHandler содержит обработчики для REST API
type APIHandler struct {
	Repository  *repository.Repository
	MinIOClient *storage.MinIOClient
	AuthHandler *AuthHandler
}

func NewAPIHandler(r *repository.Repository, minioClient *storage.MinIOClient, authHandler *AuthHandler) *APIHandler {
	return &APIHandler{
		Repository:  r,
		MinIOClient: minioClient,
		AuthHandler: authHandler,
	}
}

// ============ Вспомогательные функции ============

func (h *APIHandler) errorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, dto.ErrorResponse{
		Status:  "fail",
		Message: message,
	})
}

func (h *APIHandler) successResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	response := dto.SuccessResponse{
		Status:  "success",
		Message: message,
	}
	if data != nil {
		response.Data = data
	}
	c.JSON(statusCode, response)
}

// ============ ДОМЕН УСЛУГИ ============

// GetServices получает список услуг
// @Summary Получение списка услуг
// @Description Возвращает список всех услуг с возможностью поиска по названию
// @Tags Services
// @Produce json
// @Param query query string false "Поиск по названию услуги"
// @Success 200 {object} dto.ServiceListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/services [get]
func (h *APIHandler) GetServices(c *gin.Context) {
	searchQuery := c.Query("query")

	var services []repository.LicenseService
	var err error

	if searchQuery == "" {
		services, err = h.Repository.GetAllServices()
	} else {
		services, err = h.Repository.SearchServicesByName(searchQuery)
	}

	if err != nil {
		logrus.Error("Error getting services: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка получения услуг")
		return
	}

	// Преобразуем в DTO
	dtoServices := make([]dto.ServiceResponse, len(services))
	for i, s := range services {
		dtoServices[i] = dto.ServiceResponse{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			ImageURL:    s.ImageURL,
			BasePrice:   s.BasePrice,
			LicenseType: s.LicenseType,
		}
	}

	response := dto.ServiceListResponse{
		Services: dtoServices,
		Total:    len(dtoServices),
	}

	c.JSON(http.StatusOK, response)
}

// GetService получает одну услугу
// @Summary Получение услуги по ID
// @Description Возвращает детальную информацию об услуге
// @Tags Services
// @Produce json
// @Param id path int true "ID услуги"
// @Success 200 {object} dto.ServiceResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /api/services/{id} [get]
func (h *APIHandler) GetService(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID услуги")
		return
	}

	service, err := h.Repository.GetServiceByID(uint(id))
	if err != nil || service == nil {
		h.errorResponse(c, http.StatusNotFound, "Услуга не найдена")
		return
	}

	response := dto.ServiceResponse{
		ID:          service.ID,
		Name:        service.Name,
		Description: service.Description,
		ImageURL:    service.ImageURL,
		BasePrice:   service.BasePrice,
		LicenseType: service.LicenseType,
	}

	c.JSON(http.StatusOK, response)
}

// CreateService создает новую услугу
// @Summary Создание услуги
// @Description Создает новую услугу (только для модераторов)
// @Tags Services
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateServiceRequest true "Данные услуги"
// @Success 201 {object} dto.ServiceResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/services [post]
func (h *APIHandler) CreateService(c *gin.Context) {
	var req dto.CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	service, err := h.Repository.CreateService(req.Name, req.Description, req.LicenseType, req.BasePrice)
	if err != nil {
		logrus.Error("Error creating service: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка создания услуги")
		return
	}

	response := dto.ServiceResponse{
		ID:          service.ID,
		Name:        service.Name,
		Description: service.Description,
		ImageURL:    "rectangle-2-6.png", // Дефолтное изображение
		BasePrice:   service.BasePrice,
		LicenseType: service.LicenseType,
	}

	c.JSON(http.StatusCreated, response)
}

// UpdateService обновляет услугу
// @Summary Обновление услуги
// @Description Обновляет данные услуги (только для модераторов)
// @Tags Services
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID услуги"
// @Param request body dto.UpdateServiceRequest true "Данные для обновления"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/services/{id} [put]
func (h *APIHandler) UpdateService(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID услуги")
		return
	}

	var req dto.UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	// Проверяем существование услуги
	exists, err := h.Repository.ServiceExists(uint(id))
	if err != nil || !exists {
		h.errorResponse(c, http.StatusNotFound, "Услуга не найдена")
		return
	}

	// Подготавливаем указатели на поля
	var name, description, licenseType *string
	var basePrice *float64

	if req.Name != "" {
		name = &req.Name
	}
	if req.Description != "" {
		description = &req.Description
	}
	if req.LicenseType != "" {
		licenseType = &req.LicenseType
	}
	if req.BasePrice > 0 {
		basePrice = &req.BasePrice
	}

	err = h.Repository.UpdateService(uint(id), name, description, licenseType, basePrice)
	if err != nil {
		logrus.Error("Error updating service: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка обновления услуги")
		return
	}

	h.successResponse(c, http.StatusOK, "Услуга успешно обновлена", nil)
}

// DeleteService удаляет услугу
// @Summary Удаление услуги
// @Description Удаляет услугу (только для модераторов)
// @Tags Services
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID услуги"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/services/{id} [delete]
func (h *APIHandler) DeleteService(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID услуги")
		return
	}

	// Сначала получаем услугу чтобы удалить изображение
	service, _ := h.Repository.GetServiceByID(uint(id))
	if service != nil && service.ImageURL != "rectangle-2-6.png" && service.ImageURL != "" {
		// Удаляем изображение из MinIO
		if h.MinIOClient != nil {
			err := h.MinIOClient.DeleteFile(service.ImageURL)
			if err != nil {
				logrus.Warnf("Failed to delete image from MinIO: %v", err)
			}
		}
		h.Repository.DeleteServiceImage(uint(id))
	}

	// Логическое удаление услуги
	err = h.Repository.DeleteService(uint(id))
	if err != nil {
		logrus.Error("Error deleting service: ", err)
		h.errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Услуга успешно удалена", nil)
}

// AddServiceToOrder добавляет услугу в заявку
// @Summary Добавление услуги в заявку
// @Description Добавляет услугу в черновик заявки текущего пользователя
// @Tags Services
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID услуги"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/services/{id}/add-to-order [post]
func (h *APIHandler) AddServiceToOrder(c *gin.Context) {
	userID := getCurrentCreatorID()

	idStr := c.Param("id")
	serviceID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID услуги")
		return
	}

	// Проверяем существование услуги
	exists, err := h.Repository.ServiceExists(uint(serviceID))
	if err != nil || !exists {
		h.errorResponse(c, http.StatusNotFound, "Услуга не найдена")
		return
	}

	// Получаем или создаем черновик заявки
	order, err := h.Repository.GetDraftOrder(userID)
	if err != nil {
		// Черновика нет - создаем
		order, err = h.Repository.CreateDraftOrder(userID)
		if err != nil {
			logrus.Error("Error creating draft order: ", err)
			h.errorResponse(c, http.StatusInternalServerError, "Ошибка создания заявки")
			return
		}
	}

	// Добавляем услугу в заявку
	err = h.Repository.AddServiceToOrder(order.ID, uint(serviceID))
	if err != nil {
		logrus.Error("Error adding service to order: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка добавления услуги в заявку")
		return
	}

	h.successResponse(c, http.StatusOK, "Услуга добавлена в заявку", gin.H{
		"order_id": order.ID,
	})
}

// UploadServiceImage загружает изображение для услуги
// @Summary Загрузка изображения услуги
// @Description Загружает изображение для услуги в MinIO (только для модераторов)
// @Tags Services
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID услуги"
// @Param image formData file true "Файл изображения"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/services/{id}/image [post]
func (h *APIHandler) UploadServiceImage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID услуги")
		return
	}

	// Проверяем существование услуги
	service, err := h.Repository.GetServiceByID(uint(id))
	if err != nil || service == nil {
		h.errorResponse(c, http.StatusNotFound, "Услуга не найдена")
		return
	}

	// Получаем файл из запроса
	file, err := c.FormFile("image")
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Файл не найден в запросе")
		return
	}

	// Читаем содержимое файла
	openedFile, err := file.Open()
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка чтения файла")
		return
	}
	defer openedFile.Close()

	fileData, err := io.ReadAll(openedFile)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка чтения файла")
		return
	}

	// Удаляем старое изображение из MinIO (если есть)
	if service.ImageURL != "rectangle-2-6.png" && service.ImageURL != "" {
		if h.MinIOClient != nil {
			err := h.MinIOClient.DeleteFile(service.ImageURL)
			if err != nil {
				logrus.Warnf("Failed to delete old image %s: %v", service.ImageURL, err)
			}
		}
	}

	// Загружаем новое изображение в MinIO
	var imageURL string
	if h.MinIOClient != nil {
		imageURL, err = h.MinIOClient.UploadFile(fileData, file.Filename)
		if err != nil {
			logrus.Error("Error uploading to MinIO: ", err)
			h.errorResponse(c, http.StatusInternalServerError, "Ошибка загрузки изображения")
			return
		}
	} else {
		// Fallback если MinIO не настроен
		imageURL = "uploaded_" + file.Filename
	}

	// Обновляем URL изображения в БД
	err = h.Repository.UpdateServiceImage(uint(id), imageURL)
	if err != nil {
		logrus.Error("Error updating service image: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка обновления изображения")
		return
	}

	h.successResponse(c, http.StatusOK, "Изображение успешно загружено", gin.H{
		"image_url": imageURL,
	})
}

// ============ ДОМЕН ЗАЯВКИ ============

// GetCart получает информацию о корзине
// @Summary Получение информации о корзине
// @Description Возвращает количество услуг в черновике заявки
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.CartResponse
// @Router /api/orders/cart [get]
func (h *APIHandler) GetCart(c *gin.Context) {
	userID := getCurrentCreatorID()

	order, err := h.Repository.GetDraftOrder(userID)
	if err != nil {
		// Нет черновика - возвращаем пустую корзину
		c.JSON(http.StatusOK, dto.CartResponse{
			OrderID:      0,
			ServiceCount: 0,
		})
		return
	}

	count := h.Repository.GetOrderCount(order.ID)

	c.JSON(http.StatusOK, dto.CartResponse{
		OrderID:      order.ID,
		ServiceCount: count,
	})
}

// GetOrders получает список заявок
// @Summary Получение списка заявок
// @Description Возвращает список заявок с возможностью фильтрации по статусу и датам
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param status query string false "Фильтр по статусу"
// @Param date_from query string false "Дата начала (формат: 2006-01-02)"
// @Param date_to query string false "Дата окончания (формат: 2006-01-02)"
// @Success 200 {object} dto.OrderListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/orders [get]
func (h *APIHandler) GetOrders(c *gin.Context) {
	status := c.Query("status")
	dateFromStr := c.Query("date_from")
	dateToStr := c.Query("date_to")

	var dateFrom, dateTo *time.Time

	if dateFromStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			dateFrom = &parsed
		}
	}

	if dateToStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateToStr); err == nil {
			dateTo = &parsed
		}
	}

	orders, err := h.Repository.GetOrders(status, dateFrom, dateTo)
	if err != nil {
		logrus.Error("Error getting orders: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка получения заявок")
		return
	}

	// Преобразуем в DTO
	dtoOrders := make([]dto.OrderResponse, len(orders))
	for i, o := range orders {
		creator := "unknown"
		if o.Creator.Login != "" {
			creator = o.Creator.Login
		}

		moderator := ""
		if o.Moderator != nil && o.Moderator.Login != "" {
			moderator = o.Moderator.Login
		}

		dtoOrders[i] = dto.OrderResponse{
			ID:          o.ID,
			Status:      o.Status,
			CreatedAt:   o.CreatedAt,
			FormattedAt: o.FormattedAt,
			CompletedAt: o.CompletedAt,
			Creator:     creator,
			Moderator:   moderator,
			Users:       o.Users,
			Cores:       o.Cores,
			Period:      o.Period,
		}
	}

	response := dto.OrderListResponse{
		Orders: dtoOrders,
		Total:  len(dtoOrders),
	}

	c.JSON(http.StatusOK, response)
}

// GetOrder получает одну заявку
// @Summary Получение заявки по ID
// @Description Возвращает детальную информацию о заявке с услугами
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.OrderResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /api/orders/{id} [get]
func (h *APIHandler) GetOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	order, services, totalCost, err := h.Repository.GetOrderWithServices(uint(id))
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "Заявка не найдена")
		return
	}

	// Преобразуем услуги в DTO
	dtoServices := make([]dto.ServiceInOrderResp, len(services))
	for i, s := range services {
		dtoServices[i] = dto.ServiceInOrderResp{
			ID:           s.ID,
			Name:         s.Name,
			Description:  s.Description,
			ImageURL:     s.ImageURL,
			BasePrice:    s.BasePrice,
			LicenseType:  s.LicenseType,
			SupportLevel: s.SupportLevel,
			SubTotal:     s.SubTotal,
		}
	}

	creator := "unknown"
	if order.Creator.Login != "" {
		creator = order.Creator.Login
	}

	moderator := ""
	if order.Moderator != nil && order.Moderator.Login != "" {
		moderator = order.Moderator.Login
	}

	response := dto.OrderResponse{
		ID:          order.ID,
		Status:      order.Status,
		CreatedAt:   order.CreatedAt,
		FormattedAt: order.FormattedAt,
		CompletedAt: order.CompletedAt,
		Creator:     creator,
		Moderator:   moderator,
		Users:       order.Users,
		Cores:       order.Cores,
		Period:      order.Period,
		TotalCost:   totalCost,
		Services:    dtoServices,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateOrderFields обновляет поля заявки
// @Summary Обновление полей заявки
// @Description Обновляет количество пользователей, ядер и период для заявки
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Param request body dto.UpdateOrderFieldsRequest true "Данные для обновления"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/orders/{id} [put]
func (h *APIHandler) UpdateOrderFields(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	var req dto.UpdateOrderFieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	err = h.Repository.UpdateOrderFields(uint(id), req.Users, req.Cores, req.Period)
	if err != nil {
		logrus.Error("Error updating order fields: ", err)
		h.errorResponse(c, http.StatusBadRequest, "Ошибка обновления заявки (проверьте статус)")
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно обновлена", nil)
}

// FormatOrder формирует заявку
// @Summary Формирование заявки
// @Description Переводит заявку из статуса черновик в статус сформирован
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/orders/{id}/format [put]
func (h *APIHandler) FormatOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.FormatOrder(uint(id))
	if err != nil {
		logrus.Error("Error formatting order: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно сформирована", nil)
}

// CompleteOrder завершает заявку
// @Summary Завершение заявки
// @Description Завершает заявку модератором
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/orders/{id}/complete [put]
func (h *APIHandler) CompleteOrder(c *gin.Context) {
	moderatorID := getCurrentModeratorID()

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.CompleteOrder(uint(id), moderatorID)
	if err != nil {
		logrus.Error("Error completing order: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно завершена", nil)
}

// RejectOrder отклоняет заявку
// @Summary Отклонение заявки
// @Description Отклоняет заявку модератором
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/orders/{id}/reject [put]
func (h *APIHandler) RejectOrder(c *gin.Context) {
	moderatorID := getCurrentModeratorID()

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.RejectOrder(uint(id), moderatorID)
	if err != nil {
		logrus.Error("Error rejecting order: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно отклонена", nil)
}

// DeleteOrder удаляет заявку
// @Summary Удаление заявки
// @Description Удаляет заявку
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/orders/{id} [delete]
func (h *APIHandler) DeleteOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.DeleteOrder(uint(id))
	if err != nil {
		logrus.Error("Error deleting order: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно удалена", nil)
}

// ============ ДОМЕН М-М (Order Services) ============

// RemoveServiceFromOrder удаляет услугу из заявки
// @Summary Удаление услуги из заявки
// @Description Удаляет услугу из заявки
// @Tags Order-Services
// @Produce json
// @Security BearerAuth
// @Param order_id path int true "ID заявки"
// @Param service_id path int true "ID услуги"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/order-services/{order_id}/{service_id} [delete]
func (h *APIHandler) RemoveServiceFromOrder(c *gin.Context) {
	orderIDStr := c.Param("order_id")
	serviceIDStr := c.Param("service_id")

	orderID, err1 := strconv.ParseUint(orderIDStr, 10, 32)
	serviceID, err2 := strconv.ParseUint(serviceIDStr, 10, 32)

	if err1 != nil || err2 != nil || orderID == 0 || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверные ID")
		return
	}

	err := h.Repository.RemoveServiceFromOrder(uint(orderID), uint(serviceID))
	if err != nil {
		logrus.Error("Error removing service from order: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка удаления услуги из заявки")
		return
	}

	h.successResponse(c, http.StatusOK, "Услуга удалена из заявки", nil)
}

// UpdateOrderService обновляет коэффициент поддержки
// @Summary Обновление коэффициента поддержки
// @Description Изменяет коэффициент поддержки для услуги в заявке
// @Tags Order-Services
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param order_id path int true "ID заявки"
// @Param service_id path int true "ID услуги"
// @Param request body dto.UpdateOrderServiceRequest true "Данные для обновления"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/order-services/{order_id}/{service_id} [put]
func (h *APIHandler) UpdateOrderService(c *gin.Context) {
	orderIDStr := c.Param("order_id")
	serviceIDStr := c.Param("service_id")

	orderID, err1 := strconv.ParseUint(orderIDStr, 10, 32)
	serviceID, err2 := strconv.ParseUint(serviceIDStr, 10, 32)

	if err1 != nil || err2 != nil || orderID == 0 || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверные ID")
		return
	}

	var req dto.UpdateOrderServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	err := h.Repository.UpdateServiceSupportLevel(uint(orderID), uint(serviceID), req.SupportLevel)
	if err != nil {
		logrus.Error("Error updating support level: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка обновления коэффициента")
		return
	}

	h.successResponse(c, http.StatusOK, "Коэффициент поддержки обновлен", nil)
}

// UpdateProfile обновляет профиль пользователя
// @Summary Обновление профиля
// @Description Обновляет данные профиля пользователя
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateUserRequest true "Данные для обновления"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/profile [put]
func (h *APIHandler) UpdateProfile(c *gin.Context) {
	userID := getCurrentCreatorID()

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	var fullName, password *string
	if req.FullName != "" {
		fullName = &req.FullName
	}
	if req.Password != "" {
		password = &req.Password
	}

	err := h.Repository.UpdateUser(userID, fullName, password)
	if err != nil {
		logrus.Error("Error updating user: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка обновления профиля")
		return
	}

	h.successResponse(c, http.StatusOK, "Профиль успешно обновлен", nil)
}
