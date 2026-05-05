package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"transit-app/internal/domain"
	"transit-app/internal/response"
	"transit-app/internal/storage"
)

type reportHandler struct {
	reportUsecase domain.ReportUsecase
	storage       storage.StorageProvider
}

func NewReportHandler(router *gin.RouterGroup, ru domain.ReportUsecase, sp storage.StorageProvider) {
	handler := &reportHandler{
		reportUsecase: ru,
		storage:       sp,
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

// CreateReport menerima multipart/form-data dengan field:
//   - report_type   : TRAFFIC | ACCIDENT | CLOSURE (required)
//   - latitude      : float (required)
//   - longitude     : float (required)
//   - description   : string (opsional, max 500 char)
//   - reporter_type : "guest" | "user" (default "guest")
//   - image         : file gambar (required untuk laporan baru)
func (h *reportHandler) CreateReport(c *gin.Context) {
	// Parse multipart form — max 10 MB
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, response.Error("INVALID_FORM", "Gagal parse form data: "+err.Error()))
		return
	}

	// Bind field teks dari form ke struct input
	var input domain.CreateReportInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.Error("VALIDATION_ERROR", err.Error()))
		return
	}

	// Proses upload gambar — wajib untuk laporan baru
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error("IMAGE_REQUIRED", "Gambar bukti insiden wajib dilampirkan"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("FILE_OPEN_ERROR", "Gagal membuka file gambar"))
		return
	}
	defer file.Close()

	// Simpan file ke storage (lokal atau cloud)
	imageURL, err := h.storage.Save(file, fileHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error("STORAGE_ERROR", err.Error()))
		return
	}

	// Buat laporan melalui usecase
	ipAddress := c.ClientIP()
	if err := h.reportUsecase.CreateReport(c.Request.Context(), &input, imageURL, ipAddress); err != nil {
		if err.Error() == "RATE_LIMIT_EXCEEDED" {
			c.JSON(http.StatusTooManyRequests, response.Error("RATE_LIMIT_EXCEEDED", "Maksimal 5 laporan per jam"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusCreated, response.Success(gin.H{"message": "Laporan berhasil dibuat"}))
}

func (h *reportHandler) ConfirmReport(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error("INVALID_ID", "Format ID tidak valid"))
		return
	}

	var req domain.ConfirmReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error("VALIDATION_ERROR", err.Error()))
		return
	}

	if err := h.reportUsecase.ConfirmReport(c.Request.Context(), id, req.Action); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"message": "Konfirmasi berhasil"}))
}
