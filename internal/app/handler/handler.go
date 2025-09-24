package handler

import (
	"backend/internal/app/repository"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

func (h *Handler) GetLicenseModelDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")       // получаем id лицензионной модели из урла (то есть из /model/:id)
	id, err := strconv.Atoi(idStr) // преобразуем строку в int
	if err != nil {
		logrus.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model ID"})
		return
	}

	model, err := h.Repository.GetLicenseModelByID(id)
	if err != nil {
		logrus.Error(err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	ctx.HTML(http.StatusOK, "order.html", gin.H{
		"model":     model,
		"cartCount": h.Repository.GetCartCount(),
	})
}

func (h *Handler) GetLicenseModels(ctx *gin.Context) {
	var models []repository.LicenseModel
	var err error

	searchQuery := ctx.Query("query")               // получаем значение из поля поиска
	logrus.Infof("Search query: '%s'", searchQuery) // добавляем отладочную информацию

	if searchQuery == "" { // если поле поиска пусто, то просто получаем из репозитория все записи
		models, err = h.Repository.GetLicenseModels()
		if err != nil {
			logrus.Error(err)
		}
		logrus.Infof("Found %d models (no search)", len(models))
	} else {
		models, err = h.Repository.GetLicenseModelsByName(searchQuery) // в ином случае ищем модель по названию
		if err != nil {
			logrus.Error(err)
		}
		logrus.Infof("Found %d models for search '%s'", len(models), searchQuery)
	}

	cartCount := h.Repository.GetCartCount()

	ctx.HTML(http.StatusOK, "license_models.html", gin.H{
		"models":    models,
		"query":     searchQuery, // передаем введенный запрос обратно на страницу
		"cartCount": cartCount,
	})
}

func (h *Handler) GetLicenseCalculator(ctx *gin.Context) {
	models, err := h.Repository.GetCartModels()
	if err != nil {
		logrus.Error(err)
	}

	cartCount := h.Repository.GetCartCount()

	ctx.HTML(http.StatusOK, "license_calculator.html", gin.H{
		"models":    models,
		"cartCount": cartCount,
	})
}
