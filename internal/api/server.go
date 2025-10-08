package api

import (
	"backend/internal/app/dsn"
	"backend/internal/app/handler"
	"backend/internal/app/repository"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func StartServer() {
	log.Println("Starting server")

	// Загружаем переменные окружения
	err := godotenv.Load()
	if err != nil {
		logrus.Error("Ошибка загрузки .env файла")
	}

	// Подключаемся к базе данных
	dsnStr := dsn.FromEnv()
	repo, err := repository.New(dsnStr)
	if err != nil {
		logrus.Error("ошибка инициализации репозитория: ", err)
		return
	}

	handler := handler.NewHandler(repo)

	r := gin.Default()

	// Регистрируем статические файлы и маршруты через Handler (как в RIP-25-26)
	handler.RegisterStatic(r)
	handler.RegisterRoutes(r)

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	log.Println("Server down")
}
