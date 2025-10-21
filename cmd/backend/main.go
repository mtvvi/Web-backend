package main

import (
	"fmt"
	"log"

	"backend/internal/app/config"
	"backend/internal/app/dsn"
	"backend/internal/app/handler"
	"backend/internal/app/repository"
	"backend/internal/app/storage"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

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

	// Создаем handler и router
	hand := handler.NewHandler(repo)
	apiHand := handler.NewAPIHandler(repo, minioClient)
	router := gin.Default()

	// Регистрируем роуты (только один раз!)
	hand.RegisterStatic(router)
	hand.RegisterRoutes(router)
	apiHand.RegisterAPIRoutes(router)

	// Запускаем сервер напрямую без обертки
	serverAddress := fmt.Sprintf("%s:%d", conf.ServiceHost, conf.ServicePort)
	logrus.Infof("Starting server on %s", serverAddress)

	if err := router.Run(serverAddress); err != nil {
		logrus.Fatal(err)
	}

	log.Println("App terminated")
}
