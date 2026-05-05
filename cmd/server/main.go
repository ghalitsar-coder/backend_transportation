package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"transit-app/config"
	"transit-app/internal/delivery"
	"transit-app/internal/logger"
	"transit-app/internal/notification"
	"transit-app/internal/repository"
	"transit-app/internal/storage"
	"transit-app/internal/usecase"
)

func main() {
	cfg := config.LoadConfig()

	// Initialize Database
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Fatal("Failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("Failed to get sql.DB from gorm: %v", err)
	}
	defer sqlDB.Close()
	logger.Info("Successfully connected to PostgreSQL via GORM")

	// Initialize Repositories
	routeRepo := repository.NewRouteRepository(db)
	reportRepo := repository.NewReportRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)

	// Initialize Usecases
	routeUsecase := usecase.NewRouteUsecase(routeRepo)
	reportUsecase := usecase.NewReportUsecase(reportRepo)

	// Initialize Storage — simpan gambar insiden ke direktori lokal.
	// URL yang dikembalikan akan di-serve oleh Gin Static() di bawah.
	localStorage, err := storage.NewLocalStorage("./uploads/reports", "/uploads/reports")
	if err != nil {
		logger.Fatal("Gagal inisialisasi storage: %v", err)
	}

	// Context for background jobs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Background Jobs
	go reportUsecase.RunAutoResolve(ctx)

	// Initialize WebSocket Server
	wsServer := notification.NewWebSocketServer(vehicleRepo)

	// Load GeoJSON polylines untuk simulator kendaraan.
	// routes.geojson adalah output dari convert-csv-to-geojson.js yang
	// disimpan di voyage-planner/public/. Backend membacanya satu kali saat startup
	// dan menyimpannya ke in-memory cache.
	exeDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	geojsonCandidates := []string{
		// Relative dari working directory backend_transportation/
		filepath.Join("..", "voyage-planner", "public", "routes.geojson"),
		// Relative dari tmp/ (tempat air menyimpan binary)
		filepath.Join(exeDir, "..", "..", "voyage-planner", "public", "routes.geojson"),
		// Absolute path untuk development lokal
		"/home/ghalytsar/Kuliah/CLOUD/EVALUASI_2/voyage-planner/public/routes.geojson",
	}
	// Prioritaskan env variable jika di-set
	if envPath := os.Getenv("GEOJSON_ROUTES_PATH"); envPath != "" {
		geojsonCandidates = append([]string{envPath}, geojsonCandidates...)
	}

	geojsonLoaded := false
	for _, candidate := range geojsonCandidates {
		absCandidate, _ := filepath.Abs(candidate)
		logger.Info("Trying GeoJSON path: %s", absCandidate)
		if err := wsServer.LoadGeoJSONRoutes(absCandidate); err == nil {
			logger.Info("GeoJSON routes loaded from: %s", absCandidate)
			geojsonLoaded = true
			break
		}
	}
	if !geojsonLoaded {
		logger.Error("WARNING: routes.geojson tidak ditemukan. Set GEOJSON_ROUTES_PATH env var.")
	}

	go wsServer.RunSimulator(ctx)

	// Initialize Gin Engine
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS Middleware — diperlukan agar Vite dev server (port 5173) bisa akses backend
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Setup API group
	api := router.Group("/api")

	// Register HTTP Handlers
	delivery.NewRouteHandler(api, routeUsecase)
	delivery.NewReportHandler(api, reportUsecase, localStorage)

	// Serve file gambar yang di-upload oleh user
	// URL: GET /uploads/reports/<filename.jpg>
	router.Static("/uploads", "./uploads")

	// Register WebSocket route
	wsServer.RegisterRoutes(router)

	// Setup HTTP Server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	cancel() // Stop background jobs

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exiting")
}
