package handler

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"backend/internal/app/config"
	"backend/internal/app/ds"
	"backend/internal/app/dto"
	"backend/internal/app/redis"
	"backend/internal/app/repository"
	"backend/internal/app/role"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/sirupsen/logrus"
)

type AuthHandler struct {
	Repository  *repository.Repository
	RedisClient *redis.Client
	Config      *config.Config
}

func NewAuthHandler(r *repository.Repository, redisClient *redis.Client, config *config.Config) *AuthHandler {
	return &AuthHandler{
		Repository:  r,
		RedisClient: redisClient,
		Config:      config,
	}
}

// generateHashString генерирует SHA-1 хеш из строки
func generateHashString(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// RegisterUser регистрация нового пользователя
// @Summary Регистрация пользователя
// @Description Создание нового пользователя в системе
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Данные для регистрации"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/auth/register [post]
func (h *AuthHandler) RegisterUser(ctx *gin.Context) {
	var request dto.RegisterRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Проверяем существует ли пользователь
	exists, _ := h.Repository.UserExistsByLogin(request.Login)
	if exists {
		h.errorHandler(ctx, http.StatusBadRequest, errors.New("пользователь с таким логином уже существует"))
		return
	}

	// Хешируем пароль
	hashedPassword := generateHashString(request.Password)

	// Определяем роль (если не указана, по умолчанию Buyer)
	userRole := request.Role
	if userRole < 0 || userRole > 2 {
		userRole = 0 // Buyer по умолчанию
	}

	user, err := h.Repository.CreateUser(request.Login, hashedPassword, request.FullName, userRole)
	if err != nil {
		logrus.Error("Error creating user: ", err)
		h.errorHandler(ctx, http.StatusInternalServerError, errors.New("ошибка регистрации пользователя"))
		return
	}

	// Генерируем JWT токен сразу при регистрации
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, ds.JWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(h.Config.JWT.ExpiresIn).Unix(),
			IssuedAt:  now.Unix(),
			Issuer:    "license-calculator",
		},
		UserID: user.ID,
		Role:   role.Role(user.Role),
	})

	accessToken, err := token.SignedString([]byte(h.Config.JWT.Token))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	response := dto.UserResponse{
		ID:          user.ID,
		Login:       user.Login,
		FullName:    user.FullName,
		IsModerator: user.IsModerator,
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "пользователь успешно зарегистрирован",
		"user":    response,
		"data":    accessToken, // JWT токен
	})
}

// LoginUser аутентификация пользователя
// @Summary Вход в систему
// @Description Аутентификация пользователя с возвратом JWT токена
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Данные для входа"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/auth/login [post]
func (h *AuthHandler) LoginUser(ctx *gin.Context) {
	var request dto.LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Хешируем входной пароль
	hashedPassword := generateHashString(request.Password)

	// Проверяем пользователя в базе данных
	user, err := h.Repository.GetUserByLogin(request.Login)
	if err != nil || user.Password != hashedPassword {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("неверный логин или пароль"))
		return
	}

	// Берём роль из базы данных
	userRole := role.Role(user.Role)

	// Создание JWT токена
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, ds.JWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(h.Config.JWT.ExpiresIn).Unix(),
			IssuedAt:  now.Unix(),
			Issuer:    "license-calculator",
		},
		UserID: user.ID,
		Role:   userRole,
	})

	// Подписываем токен
	accessToken, err := token.SignedString([]byte(h.Config.JWT.Token))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "пользователь успешно авторизован",
		"user_id":    user.ID,
		"role":       int(userRole),
		"token":      accessToken,
		"login":      user.Login,
		"expires_in": int(h.Config.JWT.ExpiresIn.Seconds()),
		"token_type": "Bearer",
	})
}

// LogoutUser выход пользователя из системы
// @Summary Выход из системы
// @Description Завершение сеанса пользователя с добавлением токена в blacklist
// @Tags Authentication
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/auth/logout [post]
func (h *AuthHandler) LogoutUser(ctx *gin.Context) {
	// Получение токена из заголовка
	tokenString := ctx.GetHeader("Authorization")
	if tokenString == "" {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("authorization header missing"))
		return
	}

	// Удаление префикса "Bearer "
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// Парсинг токена для получения TTL
	token, err := jwt.ParseWithClaims(tokenString, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.Config.JWT.Token), nil
	})

	if err != nil {
		h.errorHandler(ctx, http.StatusUnauthorized, err)
		return
	}

	claims, ok := token.Claims.(*ds.JWTClaims)
	if !ok {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("invalid token claims"))
		return
	}

	// Вычисление TTL до истечения токена
	ttl := time.Until(time.Unix(claims.ExpiresAt, 0))
	if ttl <= 0 {
		// Токен уже истек
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "пользователь успешно вышел из системы",
		})
		return
	}

	// Добавление токена в blacklist
	err = h.RedisClient.WriteJWTToBlacklist(context.Background(), tokenString, ttl)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "пользователь успешно вышел из системы",
	})
}

// SessionLoginUser аутентификация с сессиями и куки
// @Summary Вход в систему (сессии)
// @Description Аутентификация пользователя с созданием сессии и куки
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Данные для входа"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/auth/session-login [post]
func (h *AuthHandler) SessionLoginUser(ctx *gin.Context) {
	var request dto.LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	// Хешируем входной пароль
	hashedPassword := generateHashString(request.Password)

	// Проверяем пользователя в базе данных
	user, err := h.Repository.GetUserByLogin(request.Login)
	if err != nil || user.Password != hashedPassword {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("неверный логин или пароль"))
		return
	}

	// Определяем роль
	userRole := role.Buyer
	if user.IsModerator {
		userRole = role.Admin
	}

	// Создание JWT токена для сессии
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, ds.JWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(24 * time.Hour).Unix(), // сессия на 24 часа
			IssuedAt:  now.Unix(),
			Issuer:    "license-calculator",
		},
		UserID: user.ID,
		Role:   userRole,
	})

	accessToken, err := token.SignedString([]byte(h.Config.JWT.Token))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Сохраняем сессию в Redis
	sessionKey := fmt.Sprintf("session:%d", user.ID)
	err = h.RedisClient.WriteJWTToBlacklist(ctx.Request.Context(), sessionKey, 24*time.Hour)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Устанавливаем куки
	ctx.SetCookie("session_id", fmt.Sprintf("%d", user.ID), 86400, "/", "", false, true) // HttpOnly cookie
	ctx.SetCookie("auth_token", accessToken, 86400, "/", "", false, false)               // обычный cookie для фронтенда
	ctx.SetCookie("user_id", fmt.Sprintf("%d", user.ID), 86400, "/", "", false, false)

	ctx.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "пользователь успешно авторизован (сессия)",
		"user_id":    user.ID,
		"role":       int(userRole),
		"login":      user.Login,
		"session_id": fmt.Sprintf("%d", user.ID),
	})
}

// SessionLogoutUser выход из сессии
// @Summary Выход из системы (сессии)
// @Description Завершение сеанса пользователя и удаление сессии
// @Tags Authentication
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/auth/session-logout [post]
func (h *AuthHandler) SessionLogoutUser(ctx *gin.Context) {
	// Получаем session_id из куки
	sessionID, err := ctx.Cookie("session_id")
	if err != nil {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("no session found"))
		return
	}

	// Удаляем сессию из Redis (добавляем в blacklist с минимальным TTL)
	sessionKey := "session:" + sessionID
	err = h.RedisClient.WriteJWTToBlacklist(ctx.Request.Context(), sessionKey, time.Second)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Удаляем куки
	ctx.SetCookie("session_id", "", -1, "/", "", false, true)
	ctx.SetCookie("auth_token", "", -1, "/", "", false, false)
	ctx.SetCookie("user_id", "", -1, "/", "", false, false)

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "пользователь успешно вышел из системы (сессия)",
	})
}

// GetUserProfile получение профиля пользователя
// @Summary Получение профиля пользователя
// @Description Возвращает информацию о текущем пользователе
// @Tags Authentication
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/auth/profile [get]
func (h *AuthHandler) GetUserProfile(ctx *gin.Context) {
	// Получаем ID пользователя из контекста (установлен middleware)
	userID, exists := ctx.Get("userID")
	if !exists {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("пользователь не авторизован"))
		return
	}

	// Получаем роль из контекста
	userRole, roleExists := ctx.Get("userRole")
	if !roleExists {
		h.errorHandler(ctx, http.StatusUnauthorized, errors.New("роль пользователя не найдена"))
		return
	}

	// Получаем пользователя из БД для полной информации
	user, err := h.Repository.GetUserByID(userID.(uint))
	if err != nil {
		h.errorHandler(ctx, http.StatusNotFound, errors.New("пользователь не найден"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"user": gin.H{
			"id":        user.ID,
			"login":     user.Login,
			"full_name": user.FullName,
			"role":      userRole,
		},
	})
}

// errorHandler централизованная обработка ошибок
func (h *AuthHandler) errorHandler(ctx *gin.Context, errorStatusCode int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(errorStatusCode, gin.H{
		"status":      "error",
		"description": err.Error(),
	})
}
