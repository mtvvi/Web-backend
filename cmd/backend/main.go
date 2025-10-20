package main

import (
	"log"

	"backend/internal/app/config"
	"backend/internal/app/dsn"
	"backend/internal/app/handler"
	"backend/internal/app/repository"
	"backend/internal/pkg"

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

	// Создаем handler и router
	hand := handler.NewHandler(repo)
	router := gin.Default()

	// Создаем и запускаем приложение
	application := pkg.NewApp(conf, router, hand)
	application.RunApp()

	log.Println("App terminated")
}
