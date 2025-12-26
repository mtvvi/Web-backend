package handler

import (
	"backend/internal/app/middleware"
	"backend/internal/app/role"

	"github.com/gin-gonic/gin"
)

// RegisterAPIRoutes регистрирует все REST API маршруты с авторизацией
func (h *APIHandler) RegisterAPIRoutes(router *gin.Engine, authMiddleware *middleware.AuthMiddleware) {
	// REST API маршруты
	api := router.Group("/api")

	// ============ Лицензии (Services) - публичные и для модераторов ============
	services := api.Group("/services")
	{
		// Публичные эндпоинты (без авторизации)
		services.GET("", h.GetServices)    // GET список с фильтрацией
		services.GET("/:id", h.GetService) // GET одна запись

		// Для авторизованных пользователей (добавление в заявку)
		services.POST("/:id/add-to-licenseCalculationRequest", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.AddServiceToLicenseCalculationRequest)

		// Только для модераторов (управление услугами)
		services.POST("", authMiddleware.WithAuthCheck(role.Admin), h.CreateService)                // POST создание
		services.PUT("/:id", authMiddleware.WithAuthCheck(role.Admin), h.UpdateService)             // PUT изменение
		services.DELETE("/:id", authMiddleware.WithAuthCheck(role.Admin), h.DeleteService)          // DELETE удаление
		services.POST("/:id/image", authMiddleware.WithAuthCheck(role.Admin), h.UploadServiceImage) // POST изображение
	}

	// ============ Заявки (LicenseCalculationRequests) - для авторизованных пользователей ============
	licenseCalculationRequests := api.Group("/licenseCalculationRequests")
	{
		// Для всех авторизованных пользователей
		licenseCalculationRequests.GET("/cart", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.GetCart)
		licenseCalculationRequests.GET("", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.GetLicenseCalculationRequests)
		licenseCalculationRequests.GET("/:id", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.GetLicenseCalculationRequest)
		licenseCalculationRequests.PUT("/:id", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.UpdateLicenseCalculationRequestFields)
		licenseCalculationRequests.PUT("/:id/format", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.FormatLicenseCalculationRequest)
		licenseCalculationRequests.DELETE("/:id", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.DeleteLicenseCalculationRequest)

		// Только для модераторов
		licenseCalculationRequests.PUT("/:id/complete", authMiddleware.WithAuthCheck(role.Admin), h.CompleteLicenseCalculationRequest) // PUT завершить
		licenseCalculationRequests.PUT("/:id/reject", authMiddleware.WithAuthCheck(role.Admin), h.RejectLicenseCalculationRequest)     // PUT отклонить
	}

	// М-М связь (LicenseCalculationRequest Services) - для авторизованных пользователей
	licenseCalculationRequestServices := api.Group("/licenseCalculationRequest-services")
	licenseCalculationRequestServices.Use(authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin))
	{
		licenseCalculationRequestServices.DELETE("/:licenseCalculationRequest_id/:service_id", h.RemoveServiceFromLicenseCalculationRequest) // DELETE из заявки
		licenseCalculationRequestServices.PUT("/:licenseCalculationRequest_id/:service_id", h.UpdateLicensePaymentRequestService)            // PUT изменение коэффициента
	}

	// ============ Аутентификация (публичные эндпоинты) ============
	auth := api.Group("/auth")
	{
		// Публичные эндпоинты
		auth.POST("/register", h.AuthHandler.RegisterUser)            // POST регистрация
		auth.POST("/login", h.AuthHandler.LoginUser)                  // POST аутентификация JWT
		auth.POST("/session-login", h.AuthHandler.SessionLoginUser)   // POST сессионная авторизация (через cookies)
		auth.POST("/session-logout", h.AuthHandler.SessionLogoutUser) // POST выход из сессии (cookies)

		// Защищенные эндпоинты
		auth.GET("/profile", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.AuthHandler.GetUserProfile)
		auth.PUT("/profile", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.UpdateProfile) // PUT обновление профиля
		auth.POST("/logout", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.AuthHandler.LogoutUser)
	}

	// Асинхронные колбэки (псевдо-авторизация по ключу)
	async := api.Group("/async")
	{
		async.PUT("/licenseCalculationRequests/:licenseCalculationRequest_id/services/:service_id/subtotal", h.ReceiveSubtotalResult)
	}

	// Ping эндпоинт для проверки
	router.GET("/ping", h.Ping)
}

// Ping проверяет работоспособность API
// @Summary Проверка работоспособности
// @Description Возвращает простой ответ для проверки работы сервера
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /ping [get]
func (h *APIHandler) Ping(ctx *gin.Context) {
	ctx.JSON(200, gin.H{"message": "pong"})
}
