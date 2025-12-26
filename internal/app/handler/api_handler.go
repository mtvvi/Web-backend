package handler

import (
	"backend/internal/app/dto"
	"backend/internal/app/repository"
	"backend/internal/app/role"
	"backend/internal/app/storage"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// APIHandler содержит обработчики для REST API
type APIHandler struct {
	Repository  *repository.Repository
	MinIOClient *storage.MinIOClient
	AuthHandler *AuthHandler
}

const (
	// Простой псевдо-ключ для взаимодействия с асинхронным сервисом
	asyncSecretKey = "license-async-key"
	// URL асинхронного сервиса (Django) для запуска задач
	asyncServiceURL = "http://localhost:8001/api/license-activation"
	// Базовый адрес текущего API, который будет использовать async-сервис для колбэка
	mainServiceBaseURL = "http://localhost:8080"
)

// asyncTaskPayload описывает задачу, которую основной сервис отправляет в async-сервис
type asyncTaskPayload struct {
	LicenseCalculationRequestID uint    `json:"licenseCalculationRequest_id"`
	ServiceID                   uint    `json:"service_id"`
	LicenseType                 string  `json:"license_type"`
	BasePrice                   float64 `json:"base_price"`
	SupportLevel                float64 `json:"support_level"`
	Users                       int     `json:"users"`
	Cores                       int     `json:"cores"`
	Period                      int     `json:"period"`
	CallbackURL                 string  `json:"callback_url"`
	SecretKey                   string  `json:"secret_key"`
}

// subtotalResultRequest — тело запроса от async-сервиса с рассчитанным sub_total
type subtotalResultRequest struct {
	SubTotal float64 `json:"subtotal" binding:"required,gt=0"`
}

func NewAPIHandler(r *repository.Repository, minioClient *storage.MinIOClient, authHandler *AuthHandler) *APIHandler {
	return &APIHandler{
		Repository:  r,
		MinIOClient: minioClient,
		AuthHandler: authHandler,
	}
}

// Получение текущего пользователя из контекста
func (h *APIHandler) getUserFromContext(c *gin.Context) (uint, role.Role, error) {
	userID, exists := c.Get("userID")
	if !exists {
		logrus.Warn("userID not found in context")
		return 0, role.Buyer, fmt.Errorf("user not authenticated")
	}

	userRole, _ := c.Get("userRole")
	r, _ := userRole.(role.Role)

	id, ok := userID.(uint)
	if !ok {
		logrus.Errorf("getUserFromContext: invalid userID type: %T", userID)
		return 0, r, fmt.Errorf("invalid user ID")
	}

	logrus.Infof("getUserFromContext: userID=%d, role=%v", id, r)
	return id, r, nil
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
		h.errorResponse(c, http.StatusNotFound, "Лицензия не найдена")
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
		h.errorResponse(c, http.StatusNotFound, "Лицензия не найдена")
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

	h.successResponse(c, http.StatusOK, "Лицензия успешно обновлена", nil)
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

	h.successResponse(c, http.StatusOK, "Лицензия успешно удалена", nil)
}

// AddServiceToLicenseCalculationRequest добавляет услугу в заявку
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
// @Router /api/services/{id}/add-to-licenseCalculationRequest [post]
func (h *APIHandler) AddServiceToLicenseCalculationRequest(c *gin.Context) {
	userID, _, err := h.getUserFromContext(c)
	if err != nil || userID == 0 {
		h.errorResponse(c, http.StatusUnauthorized, "Ошибка авторизации")
		return
	}

	idStr := c.Param("id")
	serviceID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID услуги")
		return
	}

	// Проверяем существование услуги
	exists, err := h.Repository.ServiceExists(uint(serviceID))
	if err != nil || !exists {
		h.errorResponse(c, http.StatusNotFound, "Лицензия не найдена")
		return
	}

	// Получаем или создаем черновик заявки
	licenseCalculationRequest, err := h.Repository.GetDraftLicenseCalculationRequest(userID)
	if err != nil {
		// Черновика нет - создаем
		licenseCalculationRequest, err = h.Repository.CreateDraftLicenseCalculationRequest(userID)
		if err != nil {
			logrus.Error("Error creating draft licenseCalculationRequest: ", err)
			h.errorResponse(c, http.StatusInternalServerError, "Ошибка создания заявки")
			return
		}
	}

	// Добавляем услугу в заявку
	err = h.Repository.AddServiceToLicenseCalculationRequest(licenseCalculationRequest.ID, uint(serviceID))
	if err != nil {
		logrus.Error("Error adding service to licenseCalculationRequest: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка добавления услуги в заявку")
		return
	}

	h.successResponse(c, http.StatusOK, "Лицензия добавлена в заявку", gin.H{
		"licenseCalculationRequest_id": licenseCalculationRequest.ID,
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
		h.errorResponse(c, http.StatusNotFound, "Лицензия не найдена")
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
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.CartResponse
// @Router /api/licenseCalculationRequests/cart [get]
func (h *APIHandler) GetCart(c *gin.Context) {
	userID, _, err := h.getUserFromContext(c)
	if err != nil || userID == 0 {
		// Нет авторизации - возвращаем пустую корзину
		c.JSON(http.StatusOK, dto.CartResponse{
			LicenseCalculationRequestID: 0,
			ServiceCount:                0,
		})
		return
	}

	licenseCalculationRequest, err := h.Repository.GetDraftLicenseCalculationRequest(userID)
	if err != nil {
		// Нет черновика - возвращаем пустую корзину
		c.JSON(http.StatusOK, dto.CartResponse{
			LicenseCalculationRequestID: 0,
			ServiceCount:                0,
		})
		return
	}

	count := h.Repository.GetLicenseCalculationRequestCount(licenseCalculationRequest.ID)

	c.JSON(http.StatusOK, dto.CartResponse{
		LicenseCalculationRequestID: licenseCalculationRequest.ID,
		ServiceCount:                count,
	})
}

// GetLicenseCalculationRequests получает список заявок
// @Summary Получение списка заявок
// @Description Возвращает список заявок с возможностью фильтрации по статусу и датам
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Param status query string false "Фильтр по статусу"
// @Param date_from query string false "Дата начала (формат: 2006-01-02)"
// @Param date_to query string false "Дата окончания (формат: 2006-01-02)"
// @Success 200 {object} dto.LicenseCalculationRequestListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests [get]
func (h *APIHandler) GetLicenseCalculationRequests(c *gin.Context) {
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

	// Получаем текущего пользователя и его роль
	userID, userRole, err := h.getUserFromContext(c)
	if err != nil {
		logrus.Error("Error getting user from context: ", err)
		h.errorResponse(c, http.StatusUnauthorized, "Ошибка авторизации")
		return
	}

	// Если пользователь - обычный Buyer, показываем только его заявки
	var creatorID *uint
	if userRole == role.Buyer {
		creatorID = &userID
	}

	licenseCalculationRequests, err := h.Repository.GetLicenseCalculationRequests(status, dateFrom, dateTo, creatorID)
	if err != nil {
		logrus.Error("Error getting licenseCalculationRequests: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка получения заявок")
		return
	}

	// Преобразуем в DTO
	dtoLicenseCalculationRequests := make([]dto.LicenseCalculationRequestResponse, len(licenseCalculationRequests))
	for i, o := range licenseCalculationRequests {
		creator := "unknown"
		if o.Creator.Login != "" {
			creator = o.Creator.Login
		}

		moderator := ""
		if o.Moderator != nil && o.Moderator.Login != "" {
			moderator = o.Moderator.Login
		}

		readyCount := h.Repository.CountCalculatedServices(o.ID)

		dtoLicenseCalculationRequests[i] = dto.LicenseCalculationRequestResponse{
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
			TotalCost:   o.TotalCost,
			ReadyCount:  readyCount,
		}
	}

	response := dto.LicenseCalculationRequestListResponse{
		LicenseCalculationRequests: dtoLicenseCalculationRequests,
		Total:                      len(dtoLicenseCalculationRequests),
	}

	c.JSON(http.StatusOK, response)
}

// GetLicenseCalculationRequest получает одну заявку
// @Summary Получение заявки по ID
// @Description Возвращает детальную информацию о заявке с услугами
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.LicenseCalculationRequestResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests/{id} [get]
func (h *APIHandler) GetLicenseCalculationRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	licenseCalculationRequest, services, totalCost, err := h.Repository.GetLicenseCalculationRequestWithServices(uint(id))
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "Заявка не найдена")
		return
	}

	// Преобразуем услуги в DTO
	dtoServices := make([]dto.ServiceInLicenseCalculationRequestResp, len(services))
	for i, s := range services {
		dtoServices[i] = dto.ServiceInLicenseCalculationRequestResp{
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
	if licenseCalculationRequest.Creator.Login != "" {
		creator = licenseCalculationRequest.Creator.Login
	}

	moderator := ""
	if licenseCalculationRequest.Moderator != nil && licenseCalculationRequest.Moderator.Login != "" {
		moderator = licenseCalculationRequest.Moderator.Login
	}

	response := dto.LicenseCalculationRequestResponse{
		ID:          licenseCalculationRequest.ID,
		Status:      licenseCalculationRequest.Status,
		CreatedAt:   licenseCalculationRequest.CreatedAt,
		FormattedAt: licenseCalculationRequest.FormattedAt,
		CompletedAt: licenseCalculationRequest.CompletedAt,
		Creator:     creator,
		Moderator:   moderator,
		Users:       licenseCalculationRequest.Users,
		Cores:       licenseCalculationRequest.Cores,
		Period:      licenseCalculationRequest.Period,
		TotalCost:   totalCost,
		Services:    dtoServices,
		ReadyCount:  h.Repository.CountCalculatedServices(licenseCalculationRequest.ID),
	}

	c.JSON(http.StatusOK, response)
}

// UpdateLicenseCalculationRequestFields обновляет поля заявки
// @Summary Обновление полей заявки
// @Description Обновляет количество пользователей, ядер и период для заявки
// @Tags LicenseCalculationRequests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Param request body dto.UpdateLicenseCalculationRequestFieldsRequest true "Данные для обновления"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests/{id} [put]
func (h *APIHandler) UpdateLicenseCalculationRequestFields(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	var req dto.UpdateLicenseCalculationRequestFieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	err = h.Repository.UpdateLicenseCalculationRequestFields(uint(id), req.Users, req.Cores, req.Period)
	if err != nil {
		logrus.Error("Error updating licenseCalculationRequest fields: ", err)
		h.errorResponse(c, http.StatusBadRequest, "Ошибка обновления заявки (проверьте статус)")
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно обновлена", nil)
}

// FormatLicenseCalculationRequest формирует заявку
// @Summary Формирование заявки
// @Description Переводит заявку из статуса черновик в статус сформирован
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests/{id}/format [put]
func (h *APIHandler) FormatLicenseCalculationRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.FormatLicenseCalculationRequest(uint(id))
	if err != nil {
		logrus.Error("Error formatting licenseCalculationRequest: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно сформирована", nil)
}

// Отправляет задачи в асинхронный сервис для расчета sub_total по услугам
func (h *APIHandler) triggerAsyncCalculation(licenseCalculationRequestID uint) {
	licenseCalculationRequest, services, _, err := h.Repository.GetLicenseCalculationRequestWithServices(licenseCalculationRequestID)
	if err != nil {
		logrus.Errorf("triggerAsyncCalculation: cannot load licenseCalculationRequest %d: %v", licenseCalculationRequestID, err)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, svc := range services {
		payload := asyncTaskPayload{
			LicenseCalculationRequestID: licenseCalculationRequestID,
			ServiceID:                   svc.ID,
			LicenseType:                 svc.LicenseType,
			BasePrice:                   svc.BasePrice,
			SupportLevel:                svc.SupportLevel,
			Users:                       licenseCalculationRequest.Users,
			Cores:                       licenseCalculationRequest.Cores,
			Period:                      licenseCalculationRequest.Period,
			CallbackURL:                 fmt.Sprintf("%s/api/async/licenseCalculationRequests/%d/services/%d/subtotal", mainServiceBaseURL, licenseCalculationRequestID, svc.ID),
			SecretKey:                   asyncSecretKey,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			logrus.Errorf("triggerAsyncCalculation: marshal error for licenseCalculationRequest %d, service %d: %v", licenseCalculationRequestID, svc.ID, err)
			continue
		}

		go func(p asyncTaskPayload, reqBody []byte) {
			req, err := http.NewRequest(http.MethodPost, asyncServiceURL, bytes.NewBuffer(reqBody))
			if err != nil {
				logrus.Errorf("triggerAsyncCalculation: build request failed: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				logrus.Errorf("triggerAsyncCalculation: request failed for licenseCalculationRequest %d, service %d: %v", p.LicenseCalculationRequestID, p.ServiceID, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 300 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				logrus.Warnf("triggerAsyncCalculation: async service responded %d: %s", resp.StatusCode, string(bodyBytes))
			}
		}(payload, body)
	}
}

// CompleteLicenseCalculationRequest завершает заявку
// @Summary Завершение заявки
// @Description Завершает заявку модератором
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests/{id}/complete [put]
func (h *APIHandler) CompleteLicenseCalculationRequest(c *gin.Context) {
	moderatorID, _, err := h.getUserFromContext(c)
	if err != nil || moderatorID == 0 {
		h.errorResponse(c, http.StatusUnauthorized, "Ошибка авторизации")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.CompleteLicenseCalculationRequest(uint(id), moderatorID)
	if err != nil {
		logrus.Error("Error completing licenseCalculationRequest: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Обнуляем sub_total и запускаем асинхронный расчет результатов для услуг в заявке
	if err := h.Repository.ResetLicenseCalculationRequestSubTotals(uint(id)); err != nil {
		logrus.Error("Error resetting subtotals: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Не удалось сбросить промежуточные суммы")
		return
	}
	h.triggerAsyncCalculation(uint(id))

	h.successResponse(c, http.StatusOK, "Заявка успешно завершена", nil)
}

// RejectLicenseCalculationRequest отклоняет заявку
// @Summary Отклонение заявки
// @Description Отклоняет заявку модератором
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests/{id}/reject [put]
func (h *APIHandler) RejectLicenseCalculationRequest(c *gin.Context) {
	moderatorID, _, err := h.getUserFromContext(c)
	if err != nil || moderatorID == 0 {
		h.errorResponse(c, http.StatusUnauthorized, "Ошибка авторизации")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.RejectLicenseCalculationRequest(uint(id), moderatorID)
	if err != nil {
		logrus.Error("Error rejecting licenseCalculationRequest: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно отклонена", nil)
}

// DeleteLicenseCalculationRequest удаляет заявку
// @Summary Удаление заявки
// @Description Удаляет заявку
// @Tags LicenseCalculationRequests
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID заявки"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequests/{id} [delete]
func (h *APIHandler) DeleteLicenseCalculationRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверный ID заявки")
		return
	}

	err = h.Repository.DeleteLicenseCalculationRequest(uint(id))
	if err != nil {
		logrus.Error("Error deleting licenseCalculationRequest: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Заявка успешно удалена", nil)
}

// ReceiveSubtotalResult принимает результат асинхронного сервиса
// @Summary Прием результата асинхронного расчета
// @Description Принимает рассчитанный sub_total по услуге от внешнего async сервиса (по секретному ключу)
// @Tags LicenseCalculationRequests
// @Accept json
// @Produce json
// @Param licenseCalculationRequest_id path int true "ID заявки"
// @Param service_id path int true "ID услуги"
// @Param request body subtotalResultRequest true "Рассчитанный sub_total"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Router /api/async/licenseCalculationRequests/{licenseCalculationRequest_id}/services/{service_id}/subtotal [put]
func (h *APIHandler) ReceiveSubtotalResult(c *gin.Context) {
	// Псевдо-авторизация через статичный ключ
	if c.GetHeader("X-Async-Key") != asyncSecretKey {
		h.errorResponse(c, http.StatusUnauthorized, "Неверный async ключ")
		return
	}

	licenseCalculationRequestIDStr := c.Param("licenseCalculationRequest_id")
	serviceIDStr := c.Param("service_id")

	licenseCalculationRequestID, err1 := strconv.ParseUint(licenseCalculationRequestIDStr, 10, 32)
	serviceID, err2 := strconv.ParseUint(serviceIDStr, 10, 32)
	if err1 != nil || err2 != nil || licenseCalculationRequestID == 0 || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверные ID")
		return
	}

	var req subtotalResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	if err := h.Repository.UpdateLicenseCalculationRequestSubTotal(uint(licenseCalculationRequestID), uint(serviceID), req.SubTotal); err != nil {
		logrus.Error("ReceiveSubtotalResult: ", err)
		h.errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.successResponse(c, http.StatusOK, "Результат сохранен", nil)
}

// ============ ДОМЕН М-М (LicenseCalculationRequest Services) ============

// RemoveServiceFromLicenseCalculationRequest удаляет услугу из заявки
// @Summary Удаление услуги из заявки
// @Description Удаляет услугу из заявки
// @Tags LicenseCalculationRequest-Services
// @Produce json
// @Security BearerAuth
// @Param licenseCalculationRequest_id path int true "ID заявки"
// @Param service_id path int true "ID услуги"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequest-services/{licenseCalculationRequest_id}/{service_id} [delete]
func (h *APIHandler) RemoveServiceFromLicenseCalculationRequest(c *gin.Context) {
	licenseCalculationRequestIDStr := c.Param("licenseCalculationRequest_id")
	serviceIDStr := c.Param("service_id")

	licenseCalculationRequestID, err1 := strconv.ParseUint(licenseCalculationRequestIDStr, 10, 32)
	serviceID, err2 := strconv.ParseUint(serviceIDStr, 10, 32)

	if err1 != nil || err2 != nil || licenseCalculationRequestID == 0 || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверные ID")
		return
	}

	err := h.Repository.RemoveServiceFromLicenseCalculationRequest(uint(licenseCalculationRequestID), uint(serviceID))
	if err != nil {
		logrus.Error("Error removing service from licenseCalculationRequest: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка удаления услуги из заявки")
		return
	}

	h.successResponse(c, http.StatusOK, "Лицензия удалена из заявки", nil)
}

// UpdateLicensePaymentRequestService обновляет коэффициент поддержки
// @Summary Обновление коэффициента поддержки
// @Description Изменяет коэффициент поддержки для услуги в заявке
// @Tags LicenseCalculationRequest-Services
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param licenseCalculationRequest_id path int true "ID заявки"
// @Param service_id path int true "ID услуги"
// @Param request body dto.UpdateLicensePaymentRequestServiceRequest true "Данные для обновления"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/licenseCalculationRequest-services/{licenseCalculationRequest_id}/{service_id} [put]
func (h *APIHandler) UpdateLicensePaymentRequestService(c *gin.Context) {
	licenseCalculationRequestIDStr := c.Param("licenseCalculationRequest_id")
	serviceIDStr := c.Param("service_id")

	licenseCalculationRequestID, err1 := strconv.ParseUint(licenseCalculationRequestIDStr, 10, 32)
	serviceID, err2 := strconv.ParseUint(serviceIDStr, 10, 32)

	if err1 != nil || err2 != nil || licenseCalculationRequestID == 0 || serviceID == 0 {
		h.errorResponse(c, http.StatusBadRequest, "Неверные ID")
		return
	}

	var req dto.UpdateLicensePaymentRequestServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Неверные данные: "+err.Error())
		return
	}

	err := h.Repository.UpdateServiceSupportLevel(uint(licenseCalculationRequestID), uint(serviceID), req.SupportLevel)
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
	userID, _, err := h.getUserFromContext(c)
	if err != nil || userID == 0 {
		h.errorResponse(c, http.StatusUnauthorized, "Ошибка авторизации")
		return
	}

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

	err = h.Repository.UpdateUser(userID, fullName, password)
	if err != nil {
		logrus.Error("Error updating user: ", err)
		h.errorResponse(c, http.StatusInternalServerError, "Ошибка обновления профиля")
		return
	}

	h.successResponse(c, http.StatusOK, "Профиль успешно обновлен", nil)
}
