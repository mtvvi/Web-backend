package handler

import (
	"backend/internal/app/repository"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	draftOrderID := h.Repository.GetDraftOrderID(userID)

	ctx.HTML(http.StatusOK, "license_models.html", gin.H{
		"models":       services,
		"query":        searchQuery,
		"cartCount":    cartCount,
		"draftOrderID": draftOrderID, // 0 если нет черновика
	})
}

// 2. Детали одной услуги
func (h *Handler) GetLicenseModelDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "order.html", gin.H{"error": "Invalid model ID"})
		return
	}

	service, err := h.Repository.GetServiceByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "order.html", gin.H{"error": "Model not found"})
		return
	}

	userID := uint(1)
	draftOrderID := h.Repository.GetDraftOrderID(userID)

	ctx.HTML(http.StatusOK, "order.html", gin.H{
		"model":        service,
		"cartCount":    h.Repository.GetCartCount(),
		"draftOrderID": draftOrderID,
	})
}

// 3. Калькулятор - показываем услуги из заявки
func (h *Handler) GetLicenseCalculator(ctx *gin.Context) {
	idStr := ctx.Param("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil || orderID == 0 {
		ctx.HTML(http.StatusBadRequest, "license_calculator.html", gin.H{"error": "Неверный ID заявки"})
		return
	}

	// Проверяем что заявка существует и не удалена
	order, err := h.Repository.GetOrderByID(uint(orderID))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "license_calculator.html", gin.H{"error": "Заявка не найдена или удалена"})
		return
	}

	// Получаем услуги в этой заявке
	services, err := h.Repository.GetServicesInOrder(order.ID)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "license_calculator.html", gin.H{"error": "Ошибка загрузки услуг"})
		return
	}

	// Получаем средние значения параметров из существующих записей (для отображения в форме)
	users := 0
	cores := 0
	period := 1
	if len(services) > 0 {
		for _, s := range services {
			if s.Users > users {
				users = s.Users
			}
			if s.Cores > cores {
				cores = s.Cores
			}
			if s.Period > period {
				period = s.Period
			}
		}
	}

	// Считаем итоговую стоимость
	var totalCost float64
	for _, service := range services {
		totalCost += service.SubTotal
	}

	count := h.Repository.GetOrderCount(order.ID)

	ctx.HTML(http.StatusOK, "license_calculator.html", gin.H{
		"services":  services,
		"count":     count,
		"orderID":   order.ID,
		"totalCost": totalCost,
		"users":     users,
		"cores":     cores,
		"period":    period,
	})
}

// Добавление модели в корзину (СОЗДАЕТ черновик если его нет)
func (h *Handler) AddModelToCart(ctx *gin.Context) {
	userID := uint(1)

	// Получаем или СОЗДАЕМ черновик заявки
	order, err := h.Repository.GetDraftOrder(userID)
	if err != nil {
		// Черновика нет - создаем новый
		order, err = h.Repository.CreateDraftOrder(userID)
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

	// Добавляем с параметрами по умолчанию (пользователь настроит в калькуляторе)
	err = h.Repository.AddServiceToOrder(order.ID, uint(modelID), 0, 0, 0)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Redirect(http.StatusFound, "/license-models")
}

// Удаление заявки
func (h *Handler) DeleteLicenseRequest(ctx *gin.Context) {
	idStr := ctx.Param("id")
	orderID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	err = h.Repository.DeleteOrder(uint(orderID))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Redirect(http.StatusFound, "/license-models")
}

// Обновление параметров в калькуляторе
func (h *Handler) UpdateCalculatorParams(ctx *gin.Context) {
	idStr := ctx.Param("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Получаем параметры из формы
	users := 1
	cores := 1
	period := 1

	if u := ctx.PostForm("users"); u != "" {
		if val, err := strconv.Atoi(u); err == nil && val > 0 {
			users = val
		}
	}
	if c := ctx.PostForm("cores"); c != "" {
		if val, err := strconv.Atoi(c); err == nil && val > 0 {
			cores = val
		}
	}
	if p := ctx.PostForm("period"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			period = val
		}
	}

	// Получаем все services в заявке
	services, err := h.Repository.GetServicesInOrder(uint(orderID))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Обновляем каждую услугу с новыми параметрами
	// Коэффициент поддержки остается неизменным (фиксированный)
	for _, service := range services {
		err = h.Repository.UpdateServiceInOrder(service.OrderServiceID, users, cores, period, service.SupportLevel)
		if err != nil {
			logrus.Error(err)
		}
	}

	ctx.Redirect(http.StatusFound, fmt.Sprintf("/license-calculator/%d", orderID))
}
