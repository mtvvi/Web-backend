package middleware

import (
	"sync"

	"github.com/gin-gonic/gin"
) // CurrentUser содержит информацию о текущем пользователе (singleton)
type CurrentUser struct {
	ID          uint
	Login       string
	IsModerator bool
}

var (
	currentUserInstance *CurrentUser
	once                sync.Once
)

// GetCurrentUser возвращает singleton текущего пользователя (фиксированный ID=1)
func GetCurrentUser() *CurrentUser {
	once.Do(func() {
		currentUserInstance = &CurrentUser{
			ID:          1,
			Login:       "creator1", // Фиксированный создатель
			IsModerator: false,
		}
	})
	return currentUserInstance
}

// GetModeratorUser возвращает singleton модератора (фиксированный ID=2)
func GetModeratorUser() *CurrentUser {
	return &CurrentUser{
		ID:          2,
		Login:       "moderator1",
		IsModerator: true,
	}
}

// CurrentUserMiddleware добавляет текущего пользователя в контекст
func CurrentUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser()
		c.Set("current_user", user)
		c.Next()
	}
}

// GetUserFromContext извлекает пользователя из контекста
func GetUserFromContext(c *gin.Context) *CurrentUser {
	if user, exists := c.Get("current_user"); exists {
		if u, ok := user.(*CurrentUser); ok {
			return u
		}
	}
	return GetCurrentUser() // Fallback
}
