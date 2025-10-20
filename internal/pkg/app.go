package pkg

import (
	"fmt"

	"backend/internal/app/config"
	"backend/internal/app/handler"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Application struct {
	Config  *config.Config
	Router  *gin.Engine
	Handler *handler.Handler
}

func NewApp(c *config.Config, r *gin.Engine, h *handler.Handler) *Application {
	return &Application{
		Config:  c,
		Router:  r,
		Handler: h,
	}
}

func (a *Application) RunApp() {
	logrus.Info("Server start up")

	// Регистрируем статические файлы и маршруты
	a.Handler.RegisterStatic(a.Router)
	a.Handler.RegisterRoutes(a.Router)

	serverAddress := fmt.Sprintf("%s:%d", a.Config.ServiceHost, a.Config.ServicePort)
	logrus.Infof("Starting server on %s", serverAddress)

	if err := a.Router.Run(serverAddress); err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Server down")
}
