package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"transit-app/internal/domain"
	"transit-app/internal/response"
)

type reportHandler struct {
	reportUsecase domain.ReportUsecase
}

func NewReportHandler(router *gin.RouterGroup, ru domain.ReportUsecase) {
	handler := &reportHandler{
		reportUsecase: ru,
	}

	router.GET("/reports/active", handler.GetActiveReports)
	router.POST("/reports", handler.CreateReport)
	router.POST("/reports/:id/confirm", handler.ConfirmReport)
}

func (h *reportHandler) GetActiveReports(c *gin.Context) {
	reports, err := h.reportUsecase.GetActiveReports(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(reports))
}

func (h *reportHandler) CreateReport(c *gin.Context) {
	var req domain.CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error("VALIDATION_ERROR", err.Error()))
		return
	}

	ipAddress := c.ClientIP()

	err := h.reportUsecase.CreateReport(c.Request.Context(), &req, ipAddress)
	if err != nil {
		if err.Error() == "RATE_LIMIT_EXCEEDED" {
			c.JSON(http.StatusTooManyRequests, response.Error("RATE_LIMIT_EXCEEDED", "Maximum 5 reports per hour allowed"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusCreated, response.Success(gin.H{"message": "Report created successfully"}))
}

func (h *reportHandler) ConfirmReport(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error("INVALID_ID", "Invalid report ID format"))
		return
	}

	var req domain.ConfirmReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error("VALIDATION_ERROR", err.Error()))
		return
	}

	err = h.reportUsecase.ConfirmReport(c.Request.Context(), id, req.Action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"message": "Report confirmed successfully"}))
}
