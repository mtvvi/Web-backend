package handler

import (
	"backend/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

// Регистрация маршрутов (строго по требованию: 3 GET + 2 POST)
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	// 3 GET маршрута
	router.GET("/model/:id", h.GetLicenseModelDetail)         // 1. Просмотр отдельной услуги
	router.GET("/license-models", h.GetLicenseModels)         // 2. Получение и поиск услуг
	router.GET("/license-calculator", h.GetLicenseCalculator) // 3. Просмотр текущей заявки

	// 2 POST маршрута
	router.POST("/model/:id/add-to-cart", h.AddModelToCart)    // 1. Добавление услуги в заявку (ORM)
	router.POST("/request/:id/delete", h.DeleteLicenseRequest) // 2. Логическое удаление заявки (SQL)
} // Регистрация статических файлов
func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

// Централизованная обработка ошибок (как в RIP-25-26)
func (h *Handler) errorHandler(ctx *gin.Context, errorStatusCode int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(errorStatusCode, gin.H{
		"status":      "error",
		"description": err.Error(),
	})
}
