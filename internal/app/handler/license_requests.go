package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Soft delete заявки через SQL (как требует задание)
func (h *Handler) DeleteLicenseRequest(ctx *gin.Context) {
	idStr := ctx.Param("id")
	orderID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	err = h.Repository.DeleteOrder(uint(orderID))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Redirect(http.StatusFound, "/license-models")
}
