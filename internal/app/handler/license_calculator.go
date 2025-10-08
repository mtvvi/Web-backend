package handler

import (
	"backend/internal/app/ds"
	"backend/internal/app/repository"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Структура для хранения параметров расчета
type CalculationParams struct {
	UserCount     int
	CoreCount     int
	LicensePeriod int
	SupportLevel  string
	CompanyName   string
	SupportCoeff  float64
}

func (h *Handler) GetLicenseCalculator(ctx *gin.Context) {
	// Получаем данные заказа
	order, orderServices, err := h.getOrderData(ctx)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "license_calculator.html", gin.H{"error": err.Error()})
		return
	}

	// Получаем параметры расчета
	params := h.parseCalculationParams(ctx, order)

	// Выполняем расчеты
	licenseParameters, selectedModels, totalCost := h.calculateLicenses(orderServices, params, ctx)

	// Формируем ответ
	h.renderCalculatorResponse(ctx, order, params, licenseParameters, selectedModels, totalCost)
}

// Получение данных заказа
func (h *Handler) getOrderData(ctx *gin.Context) (*ds.LicenseOrder, []ds.OrderService, error) {
	userID := uint(1)
	order, err := h.Repository.GetOrCreateDraftOrder(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order: %v", err)
	}

	orderServices, err := h.Repository.GetOrderServices(order.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order services: %v", err)
	}

	return order, orderServices, nil
}

// Парсинг параметров расчета
func (h *Handler) parseCalculationParams(ctx *gin.Context, order *ds.LicenseOrder) CalculationParams {
	params := CalculationParams{
		UserCount:     50,
		CoreCount:     40,
		LicensePeriod: 1,
		SupportLevel:  "standard",
		CompanyName:   "",
	}

	// Используем данные из заказа как базовые
	if order.LicensePeriod > 0 {
		params.LicensePeriod = order.LicensePeriod
	}
	if order.CompanyName != "" {
		params.CompanyName = order.CompanyName
	}

	// Парсим GET параметры
	if users := ctx.Query("users"); users != "" {
		if parsedUsers, err := strconv.Atoi(users); err == nil {
			params.UserCount = parsedUsers
		}
	}
	if cores := ctx.Query("cores"); cores != "" {
		if parsedCores, err := strconv.Atoi(cores); err == nil {
			params.CoreCount = parsedCores
		}
	}
	if period := ctx.Query("period"); period != "" {
		if parsedPeriod, err := strconv.Atoi(period); err == nil {
			params.LicensePeriod = parsedPeriod
		}
	}
	if support := ctx.Query("support"); support != "" {
		params.SupportLevel = support
	}
	if company := ctx.Query("company"); company != "" {
		params.CompanyName = company
	}

	// Устанавливаем коэффициент поддержки
	supportCoeffs := map[string]float64{
		"basic":    1.0,
		"standard": 1.25,
		"premium":  1.5,
	}
	params.SupportCoeff = supportCoeffs[params.SupportLevel]

	return params
}

// Выполнение расчетов для всех лицензий
func (h *Handler) calculateLicenses(orderServices []ds.OrderService, params CalculationParams, ctx *gin.Context) ([]repository.LicenseToRequest, []repository.LicenseModel, float64) {
	allModels, _ := h.Repository.GetLicenseModels()
	licenseParameters := make([]repository.LicenseToRequest, 0)
	selectedModels := make([]repository.LicenseModel, 0)
	totalCost := 0.0
	itemIndex := 0

	for _, orderService := range orderServices {
		for _, model := range allModels {
			if model.ID == int(orderService.ServiceID) {
				for i := 0; i < orderService.Quantity; i++ {
					licenseParam := h.calculateSingleLicense(model, params, itemIndex, ctx)
					licenseParameters = append(licenseParameters, licenseParam)
					selectedModels = append(selectedModels, model)
					totalCost += licenseParam.CalculatedCost
					itemIndex++
				}
				break
			}
		}
	}

	return licenseParameters, selectedModels, totalCost
}

// Расчет одной лицензии
func (h *Handler) calculateSingleLicense(model repository.LicenseModel, params CalculationParams, itemIndex int, ctx *gin.Context) repository.LicenseToRequest {
	// Получаем индивидуальный коэффициент поддержки
	itemSupportCoeff := params.SupportCoeff
	if itemSupportStr := ctx.Query(fmt.Sprintf("item_support_%d", itemIndex)); itemSupportStr != "" {
		if parsedSupport, err := strconv.ParseFloat(itemSupportStr, 64); err == nil {
			itemSupportCoeff = parsedSupport
		}
	}

	baseCostPerUnit, totalBaseCost, discountCoeff := h.calculateCostByLicenseType(model, params, itemSupportCoeff)

	calculatedCost := totalBaseCost * discountCoeff

	return repository.LicenseToRequest{
		LicenseModelId:     model.ID,
		RequiredQuantity:   1,
		CalculatedCost:     calculatedCost,
		BaseCalculatedCost: baseCostPerUnit,
		AppliedDiscount:    discountCoeff,
		SupportCoeff:       itemSupportCoeff,
	}
}

// Расчет стоимости по типу лицензии
func (h *Handler) calculateCostByLicenseType(model repository.LicenseModel, params CalculationParams, itemSupportCoeff float64) (float64, float64, float64) {
	var baseCostPerUnit, totalBaseCost float64
	discountCoeff := 1.0

	switch model.Name {
	case "Пользовательские лицензии":
		baseCostPerUnit = 16100.0
		if params.UserCount > 100 {
			discountCoeff = 0.85 // скидка 15%
		}
		totalBaseCost = baseCostPerUnit * itemSupportCoeff * float64(params.LicensePeriod) * float64(params.UserCount)

	case "Серверные лицензии":
		baseCostPerUnit = 276200.0
		if params.CoreCount > 50 {
			discountCoeff = 0.8 // скидка 20%
		}
		totalBaseCost = baseCostPerUnit * itemSupportCoeff * float64(params.LicensePeriod) * (float64(params.CoreCount) / 48.0)

	default: // Корпоративная подписка
		baseCostPerUnit = model.BasePrice
		itemSupportCoeff = 1.5 // всегда премиум для отображения
		if params.LicensePeriod > 2 {
			discountCoeff = 0.9 // скидка 10%
		}
		totalBaseCost = baseCostPerUnit * float64(params.LicensePeriod) // БЕЗ наценки за поддержку
	}

	return baseCostPerUnit, totalBaseCost, discountCoeff
}

// Формирование ответа
func (h *Handler) renderCalculatorResponse(ctx *gin.Context, order *ds.LicenseOrder, params CalculationParams, licenseParameters []repository.LicenseToRequest, selectedModels []repository.LicenseModel, totalCost float64) {
	request := &repository.LicenseRequest{
		ID:                 int(order.ID),
		CompanyName:        params.CompanyName,
		UserCount:          params.UserCount,
		CoreCount:          params.CoreCount,
		LicensePeriodYears: params.LicensePeriod,
		SupportLevel:       params.SupportLevel,
		RequestDate:        order.CreatedAt.Format("02.01.06"),
		TotalCost:          totalCost,
		LicenseParameters:  licenseParameters,
	}

	ctx.HTML(http.StatusOK, "license_calculator.html", gin.H{
		"request":   request,
		"models":    selectedModels,
		"cartCount": h.Repository.GetCartCount(),
	})
}
