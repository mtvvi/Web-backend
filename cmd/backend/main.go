package main

import (
	"context"
	"fmt"
	"log"

	_ "backend/docs"
	"backend/internal/app/config"
	"backend/internal/app/dsn"
	"backend/internal/app/handler"
	"backend/internal/app/middleware"
	"backend/internal/app/redis"
	"backend/internal/app/repository"
	"backend/internal/app/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title License Calculator API
// @version 1.0
// @description Система расчета стоимости лицензирования программного обеспечения

// @contact.name API Support
// @contact.email support@license-calculator.ru

// @license.name AS IS (NO WARRANTY)

// @host 127.0.0.1:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите JWT токен (с префиксом 'Bearer ' или без него)

func main() {
	log.Println("App start")

	// Загружаем переменные окружения
	err := godotenv.Load()
	if err != nil {
		logrus.Warn("Ошибка загрузки .env файла")
	}

	// Загружаем конфигурацию из config.toml
	conf, err := config.NewConfig()
	if err != nil {
		logrus.Fatalf("error loading config: %v", err)
	}

	// Подключаемся к базе данных
	dsnStr := dsn.FromEnv()
	repo, err := repository.New(dsnStr)
	if err != nil {
		logrus.Fatalf("error initializing repository: %v", err)
	}

	// Создаем Redis клиент
	ctx := context.Background()
	redisClient, err := redis.New(ctx, conf.Redis)
	if err != nil {
		logrus.Fatalf("error initializing redis: %v", err)
	}
	logrus.Info("Redis client initialized successfully")

	// Инициализируем MinIO клиент
	minioClient, err := storage.NewMinIOClient(
		"localhost:9000",
		"minio",
		"minio124",
		"license-images",
		false, // useSSL
	)
	if err != nil {
		logrus.Warnf("Failed to initialize MinIO client: %v", err)
		logrus.Warn("Continuing without MinIO support")
	} else {
		logrus.Info("MinIO client initialized successfully")
	}

	// Создаем router с CORS
	router := gin.Default()

	// настройка CORS
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Создаем middleware для авторизации
	authMiddleware := middleware.NewAuthMiddleware(redisClient, conf)

	// Создаем handlers
	authHandler := handler.NewAuthHandler(repo, redisClient, conf)
	apiHandler := handler.NewAPIHandler(repo, minioClient, authHandler)
	htmlHandler := handler.NewHandler(repo)

	// Настройка swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/docs/*any", func(c *gin.Context) {
		c.Redirect(302, "/swagger/index.html")
	})

	// Регистрируем роуты
	htmlHandler.RegisterStatic(router)
	htmlHandler.RegisterRoutes(router)
	apiHandler.RegisterAPIRoutes(router, authMiddleware)

	// Запускаем сервер
	serverAddress := fmt.Sprintf("%s:%d", conf.ServiceHost, conf.ServicePort)
	logrus.Infof("Starting server on %s", serverAddress)

	if err := router.Run(serverAddress); err != nil {
		logrus.Fatal(err)
	}

	log.Println("App terminated")
}
