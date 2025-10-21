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
}

func NewAPIHandler(r *repository.Repository, minioClient *storage.MinIOClient) *APIHandler {
	return &APIHandler{
		Repository:  r,
		MinIOClient: minioClient,
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

// GET /api/services - Список услуг с фильтрацией
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

// GET /api/services/:id - Одна услуга
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

// POST /api/services - Создание услуги (без изображения)
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

// PUT /api/services/:id - Изменение услуги
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

// DELETE /api/services/:id - Удаление услуги
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

// POST /api/services/:id/add-to-order - Добавление услуги в заявку-черновик
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

// POST /api/services/:id/image - Добавление изображения (MinIO)
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

// GET /api/orders/cart - Иконка корзины
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

// GET /api/orders - Список заявок с фильтрацией
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

// GET /api/orders/:id - Одна заявка с услугами
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

// PUT /api/orders/:id - Изменение полей заявки
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

// PUT /api/orders/:id/format - Сформировать заявку
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

// PUT /api/orders/:id/complete - Завершить заявку (модератором)
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

// PUT /api/orders/:id/reject - Отклонить заявку (модератором)
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

// DELETE /api/orders/:id - Удаление заявки
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

// DELETE /api/orders/:order_id/services/:service_id - Удалить услугу из заявки
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

// PUT /api/orders/:order_id/services/:service_id - Изменить коэффициент поддержки
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

// ============ ДОМЕН ПОЛЬЗОВАТЕЛИ ============

// POST /api/users/register - Регистрация
func (h *APIHandler) RegisterUser(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	// Проверяем существует ли пользователь
	exists, _ := h.Repository.UserExistsByLogin(req.Login)
	if exists {
		h.errorResponse(c, http.StatusBadRequest, "Пользователь с таким логином уже существует")
		return
	}

	user, err := h.Repository.CreateUser(req.Login, req.Password, req.FullName, req.IsModerator)
	if err != nil {
		logrus.Error("Error creating user: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка регистрации пользователя")
		return
	}

	response := dto.UserResponse{
		ID:          user.ID,
		Login:       user.Login,
		FullName:    user.FullName,
		IsModerator: user.IsModerator,
	}

	c.JSON(http.StatusCreated, response)
}

// GET /api/users/profile - Получить профиль текущего пользователя
func (h *APIHandler) GetProfile(c *gin.Context) {
	userID := getCurrentCreatorID()

	// Получаем полные данные из БД
	dbUser, err := h.Repository.GetUserByID(userID)
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "Пользователь не найден")
		return
	}

	response := dto.UserResponse{
		ID:          dbUser.ID,
		Login:       dbUser.Login,
		FullName:    dbUser.FullName,
		IsModerator: dbUser.IsModerator,
	}

	c.JSON(http.StatusOK, response)
}

// PUT /api/users/profile - Обновить профиль
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

// POST /api/users/login - Аутентификация (заглушка)
func (h *APIHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	// Простая проверка (в реальности нужна хеширование пароля)
	user, err := h.Repository.GetUserByLogin(req.Login)
	if err != nil || user.Password != req.Password {
		h.errorResponse(c, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}

	response := dto.LoginResponse{
		Token: "mock_token_" + user.Login, // Заглушка токена
		User: dto.UserResponse{
			ID:          user.ID,
			Login:       user.Login,
			FullName:    user.FullName,
			IsModerator: user.IsModerator,
		},
	}

	c.JSON(http.StatusOK, response)
}

// POST /api/users/logout - Деавторизация (заглушка)
func (h *APIHandler) Logout(c *gin.Context) {
	h.successResponse(c, http.StatusOK, "Выход выполнен успешно", nil)
}
