package middleware

import (
	"backend/internal/app/config"
	"backend/internal/app/ds"
	"backend/internal/app/redis"
	"backend/internal/app/role"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type AuthMiddleware struct {
	RedisClient *redis.Client
	Config      *config.Config
}

func NewAuthMiddleware(redisClient *redis.Client, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		RedisClient: redisClient,
		Config:      cfg,
	}
}

// WithAuthCheck middleware для проверки авторизации с ролями
func (am *AuthMiddleware) WithAuthCheck(assignedRoles ...role.Role) gin.HandlerFunc {
	return gin.HandlerFunc(func(gCtx *gin.Context) {
		// Проверяем JWT токен из заголовка Authorization
		jwtStr := gCtx.GetHeader("Authorization")
		if jwtStr == "" {
			gCtx.AbortWithStatus(401) // Unauthorized
			return
		}

		// Убираем префикс "Bearer " если он есть
		if len(jwtStr) > 7 && jwtStr[:7] == "Bearer " {
			jwtStr = jwtStr[7:]
		}

		// Проверяем токен в blacklist Redis
		err := am.RedisClient.CheckJWTInBlacklist(gCtx.Request.Context(), jwtStr)
		if err == nil {
			// Токен в blacklist
			gCtx.AbortWithStatus(401)
			return
		}

		// Парсим и проверяем JWT токен
		token, err := am.parseJWTToken(jwtStr)
		if err != nil {
			gCtx.AbortWithStatus(401)
			return
		}

		claims, ok := token.Claims.(*ds.JWTClaims)
		if !ok || !token.Valid {
			gCtx.AbortWithStatus(401)
			return
		}

		// Проверяем роли пользователя
		if len(assignedRoles) > 0 && !am.hasRequiredRole(claims.Role, assignedRoles) {
			gCtx.AbortWithStatus(403) // Forbidden
			return
		}

		// Сохраняем данные пользователя в контексте для последующего использования
		gCtx.Set("userUUID", claims.UserUUID.String())
		gCtx.Set("userRole", claims.Role)

		gCtx.Next()
	})
}

// parseJWTToken парсит и валидирует JWT токен
func (am *AuthMiddleware) parseJWTToken(tokenString string) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenString, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(am.Config.JWT.Token), nil
	})
}

// hasRequiredRole проверяет, есть ли у пользователя необходимая роль
func (am *AuthMiddleware) hasRequiredRole(userRole role.Role, requiredRoles []role.Role) bool {
	for _, requiredRole := range requiredRoles {
		if userRole == requiredRole {
			return true
		}
	}
	return false
}
