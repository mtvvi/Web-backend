package handler

import (
	"github.com/gin-gonic/gin"
)

// RegisterAPIRoutes регистрирует все REST API маршруты
func (h *APIHandler) RegisterAPIRoutes(router *gin.Engine) {
	// REST API маршруты
	api := router.Group("/api")

	// ============ Услуги (Services) ============
	services := api.Group("/services")
	{
		services.GET("", h.GetServices)                         // GET список с фильтрацией
		services.GET("/:id", h.GetService)                      // GET одна запись
		services.POST("", h.CreateService)                      // POST создание (без изображения)
		services.PUT("/:id", h.UpdateService)                   // PUT изменение
		services.DELETE("/:id", h.DeleteService)                // DELETE удаление
		services.POST("/:id/add-to-order", h.AddServiceToOrder) // POST добавление в заявку
		services.POST("/:id/image", h.UploadServiceImage)       // POST добавление изображения
	}

	// ============ Заявки (Orders) ============
	orders := api.Group("/orders")
	{
		orders.GET("/cart", h.GetCart)               // GET иконка корзины
		orders.GET("", h.GetOrders)                  // GET список с фильтрацией
		orders.GET("/:id", h.GetOrder)               // GET одна запись (+ услуги)
		orders.PUT("/:id", h.UpdateOrderFields)      // PUT изменение полей заявки
		orders.PUT("/:id/format", h.FormatOrder)     // PUT сформировать
		orders.PUT("/:id/complete", h.CompleteOrder) // PUT завершить
		orders.PUT("/:id/reject", h.RejectOrder)     // PUT отклонить
		orders.DELETE("/:id", h.DeleteOrder)         // DELETE удаление
	}

	// М-М связь (Order Services) - отдельная группа чтобы избежать конфликтов
	orderServices := api.Group("/order-services")
	{
		orderServices.DELETE("/:order_id/:service_id", h.RemoveServiceFromOrder) // DELETE из заявки
		orderServices.PUT("/:order_id/:service_id", h.UpdateOrderService)        // PUT изменение коэффициента
	}

	// ============ Пользователи (Users) ============
	users := api.Group("/users")
	{
		users.POST("/register", h.RegisterUser) // POST регистрация
		users.GET("/profile", h.GetProfile)     // GET профиль
		users.PUT("/profile", h.UpdateProfile)  // PUT обновление профиля
		users.POST("/login", h.Login)           // POST аутентификация (заглушка)
		users.POST("/logout", h.Logout)         // POST деавторизация (заглушка)
	}
}
