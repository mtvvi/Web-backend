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

	// ============ Услуги (Services) - публичные и для модераторов ============
	services := api.Group("/services")
	{
		// Публичные эндпоинты (без авторизации)
		services.GET("", h.GetServices)    // GET список с фильтрацией
		services.GET("/:id", h.GetService) // GET одна запись

		// Для авторизованных пользователей (добавление в заявку)
		services.POST("/:id/add-to-order", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.AddServiceToOrder)

		// Только для модераторов (управление услугами)
		services.POST("", authMiddleware.WithAuthCheck(role.Admin), h.CreateService)                // POST создание
		services.PUT("/:id", authMiddleware.WithAuthCheck(role.Admin), h.UpdateService)             // PUT изменение
		services.DELETE("/:id", authMiddleware.WithAuthCheck(role.Admin), h.DeleteService)          // DELETE удаление
		services.POST("/:id/image", authMiddleware.WithAuthCheck(role.Admin), h.UploadServiceImage) // POST изображение
	}

	// ============ Заявки (Orders) - для авторизованных пользователей ============
	orders := api.Group("/orders")
	{
		// Для всех авторизованных пользователей
		orders.GET("/cart", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.GetCart)
		orders.GET("", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.GetOrders)
		orders.GET("/:id", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.GetOrder)
		orders.PUT("/:id", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.UpdateOrderFields)
		orders.PUT("/:id/format", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.FormatOrder)
		orders.DELETE("/:id", authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin), h.DeleteOrder)

		// Только для модераторов
		orders.PUT("/:id/complete", authMiddleware.WithAuthCheck(role.Admin), h.CompleteOrder) // PUT завершить
		orders.PUT("/:id/reject", authMiddleware.WithAuthCheck(role.Admin), h.RejectOrder)     // PUT отклонить
	}

	// М-М связь (Order Services) - для авторизованных пользователей
	orderServices := api.Group("/order-services")
	orderServices.Use(authMiddleware.WithAuthCheck(role.Buyer, role.Manager, role.Admin))
	{
		orderServices.DELETE("/:order_id/:service_id", h.RemoveServiceFromOrder) // DELETE из заявки
		orderServices.PUT("/:order_id/:service_id", h.UpdateOrderService)        // PUT изменение коэффициента
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
		async.PUT("/orders/:order_id/services/:service_id/subtotal", h.ReceiveSubtotalResult)
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
