package handler

import (
	"backend/internal/app/repository"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetLicenseModelDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "order.html", gin.H{"error": "Invalid model ID"})
		return
	}

	model, err := h.Repository.GetLicenseModelByID(id)
	if err != nil {
		ctx.HTML(http.StatusNotFound, "order.html", gin.H{"error": "Model not found"})
		return
	}

	ctx.HTML(http.StatusOK, "order.html", gin.H{
		"model":     model,
		"cartCount": h.Repository.GetCartCount(),
	})
}

func (h *Handler) GetLicenseModels(ctx *gin.Context) {
	var models []repository.LicenseModel

	searchQuery := ctx.Query("query")

	if searchQuery == "" {
		models, _ = h.Repository.GetLicenseModels()
	} else {
		models, _ = h.Repository.GetLicenseModelsByName(searchQuery)
	}

	cartCount := h.Repository.GetCartCount()

	ctx.HTML(http.StatusOK, "license_models.html", gin.H{
		"models":    models,
		"query":     searchQuery, // передаем введенный запрос обратно на страницу
		"cartCount": cartCount,
	})
}

// Добавление модели в корзину
func (h *Handler) AddModelToCart(ctx *gin.Context) {
	userID := uint(1) // Тестовый пользователь

	// Получаем или создаем черновик заявки
	order, err := h.Repository.GetOrCreateDraftOrder(userID)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Получаем ID модели из URL
	modelIDStr := ctx.Param("id")
	modelID, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Добавляем услугу в заявку (1 единица по умолчанию)
	err = h.Repository.AddServiceToOrder(order.ID, uint(modelID), 1)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Перенаправляем обратно к каталогу
	ctx.Redirect(http.StatusFound, "/license-models")
}
