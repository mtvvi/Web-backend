package handler

import (
	"backend/internal/app/repository"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

// Регистрация статических файлов
func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

// Регистрация маршрутов
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	// GET маршруты
	router.GET("/license-models", h.GetLicenseModels)
	router.GET("/model/:id", h.GetLicenseModelDetail)
	router.GET("/license-calculator/:id", h.GetLicenseCalculator)

	// POST маршруты
	router.POST("/model/:id/add-to-cart", h.AddModelToCart)
	router.POST("/request/:id/delete", h.DeleteLicenseRequest)
	router.POST("/calculator/:id/update", h.UpdateCalculatorParams)
}

// Централизованная обработка ошибок
func (h *Handler) errorHandler(ctx *gin.Context, errorStatusCode int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(errorStatusCode, gin.H{
		"status":      "error",
		"description": err.Error(),
	})
}

// 1. Список услуг с поиском
func (h *Handler) GetLicenseModels(ctx *gin.Context) {
	var services []repository.LicenseService
	var err error

	searchQuery := ctx.Query("query")
	if searchQuery == "" {
		services, err = h.Repository.GetAllServices()
		if err != nil {
			logrus.Error(err)
		}
	} else {
		services, err = h.Repository.SearchServicesByName(searchQuery)
		if err != nil {
			logrus.Error(err)
		}
	}

	userID := uint(1)
	cartCount := h.Repository.GetCartCount()
	draftLicenseCalculationRequestID := h.Repository.GetDraftLicenseCalculationRequestID(userID)

	ctx.HTML(http.StatusOK, "license_models.html", gin.H{
		"models":                           services,
		"query":                            searchQuery,
		"cartCount":                        cartCount,
		"draftLicenseCalculationRequestID": draftLicenseCalculationRequestID, // 0 если нет черновика
	})
}

// 2. Детали одной услуги
func (h *Handler) GetLicenseModelDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "licenseCalculationRequest.html", gin.H{"error": "Invalid model ID"})
		return
	}

	service, err := h.Repository.GetServiceByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "licenseCalculationRequest.html", gin.H{"error": "Model not found"})
		return
	}

	userID := uint(1)
	draftLicenseCalculationRequestID := h.Repository.GetDraftLicenseCalculationRequestID(userID)

	ctx.HTML(http.StatusOK, "licenseCalculationRequest.html", gin.H{
		"model":                            service,
		"cartCount":                        h.Repository.GetCartCount(),
		"draftLicenseCalculationRequestID": draftLicenseCalculationRequestID,
	})
}

// 3. Калькулятор - показываем услуги из заявки
func (h *Handler) GetLicenseCalculator(ctx *gin.Context) {
	idStr := ctx.Param("id")
	licenseCalculationRequestID, err := strconv.Atoi(idStr)
	if err != nil || licenseCalculationRequestID == 0 {
		ctx.HTML(http.StatusBadRequest, "license_calculator.html", gin.H{"error": "Неверный ID заявки"})
		return
	}

	// Проверяем что заявка существует и не удалена
	licenseCalculationRequest, err := h.Repository.GetLicenseCalculationRequestByID(uint(licenseCalculationRequestID))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "license_calculator.html", gin.H{"error": "Заявка не найдена или удалена"})
		return
	}

	// Получаем услуги в этой заявке
	services, err := h.Repository.GetServicesInLicenseCalculationRequest(licenseCalculationRequest.ID)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "license_calculator.html", gin.H{"error": "Ошибка загрузки услуг"})
		return
	}

	// Параметры берем из заявки (теперь без supportLevel)
	users := licenseCalculationRequest.Users
	cores := licenseCalculationRequest.Cores
	period := licenseCalculationRequest.Period

	// Считаем итоговую стоимость
	var totalCost float64
	for _, service := range services {
		totalCost += service.SubTotal
	}

	count := h.Repository.GetLicenseCalculationRequestCount(licenseCalculationRequest.ID)

	ctx.HTML(http.StatusOK, "license_calculator.html", gin.H{
		"services":                    services,
		"count":                       count,
		"licenseCalculationRequestID": licenseCalculationRequest.ID,
		"totalCost":                   totalCost,
		"users":                       users,
		"cores":                       cores,
		"period":                      period,
	})
}

// Добавление модели в корзину (СОЗДАЕТ черновик если его нет)
func (h *Handler) AddModelToCart(ctx *gin.Context) {
	userID := uint(1)

	// Получаем или СОЗДАЕМ черновик заявки
	licenseCalculationRequest, err := h.Repository.GetDraftLicenseCalculationRequest(userID)
	if err != nil {
		// Черновика нет - создаем новый
		licenseCalculationRequest, err = h.Repository.CreateDraftLicenseCalculationRequest(userID)
		if err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	modelIDStr := ctx.Param("id")
	modelID, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Добавляем услугу в заявку (просто связь)
	err = h.Repository.AddServiceToLicenseCalculationRequest(licenseCalculationRequest.ID, uint(modelID))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Redirect(http.StatusFound, "/license-models")
}

// Удаление заявки
func (h *Handler) DeleteLicenseRequest(ctx *gin.Context) {
	idStr := ctx.Param("id")
	licenseCalculationRequestID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	err = h.Repository.DeleteLicenseCalculationRequest(uint(licenseCalculationRequestID))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Redirect(http.StatusFound, "/license-models")
}

// Обновление параметров в калькуляторе
func (h *Handler) UpdateCalculatorParams(ctx *gin.Context) {
	idStr := ctx.Param("id")
	licenseCalculationRequestID, err := strconv.Atoi(idStr)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Получаем параметры из формы
	users := 0
	cores := 0
	period := 0

	if u := ctx.PostForm("users"); u != "" {
		if val, err := strconv.Atoi(u); err == nil && val >= 0 {
			users = val
		}
	}
	if c := ctx.PostForm("cores"); c != "" {
		if val, err := strconv.Atoi(c); err == nil && val >= 0 {
			cores = val
		}
	}
	if p := ctx.PostForm("period"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val >= 0 {
			period = val
		}
	}

	// Обновляем параметры в заявке (общие для всех услуг)
	err = h.Repository.UpdateLicenseCalculationRequestParams(uint(licenseCalculationRequestID), users, cores, period)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Обновляем коэффициенты поддержки для каждой услуги
	// Получаем список всех услуг в заявке
	services, err := h.Repository.GetServicesInLicenseCalculationRequest(uint(licenseCalculationRequestID))
	if err == nil {
		for _, service := range services {
			// Ищем параметр support_level_<serviceID>
			formKey := fmt.Sprintf("support_level_%d", service.ID)
			if slStr := ctx.PostForm(formKey); slStr != "" {
				if supportLevel, err := strconv.ParseFloat(slStr, 64); err == nil {
					// service.ID - это ID из license_services
					// Обновляем коэффициент (валидация 0.7-3.0 внутри метода)
					logrus.Infof("Updating support level for licenseCalculationRequest=%d, service=%d to %.2f", licenseCalculationRequestID, service.ID, supportLevel)
					err := h.Repository.UpdateServiceSupportLevel(uint(licenseCalculationRequestID), service.ID, supportLevel)
					if err != nil {
						logrus.Errorf("Failed to update support level: %v", err)
					}
				}
			}
		}
	}

	ctx.Redirect(http.StatusFound, fmt.Sprintf("/license-calculator/%d", licenseCalculationRequestID))
}
