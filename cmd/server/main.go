package main

import (
	"context"
	"database/sql"
	"fmt"
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
	"transit-app/internal/domain"
	"transit-app/internal/logger"
	"transit-app/internal/migration"
	"transit-app/internal/notification"
	"transit-app/internal/repository"
	"transit-app/internal/storage"
	"transit-app/internal/usecase"
)

func main() {
	cfg := config.LoadConfig()

	var dbInitErr error
	var sqlDB *sql.DB

	// Initialize Database
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		dbInitErr = fmt.Errorf("gorm.Open: %w", err)
	}

	if dbInitErr == nil {
		sqlDB, err = db.DB()
		if err != nil {
			logger.Error("Failed to get sql.DB from gorm: %v", err)
			dbInitErr = fmt.Errorf("db.DB: %w", err)
		} else {
			defer sqlDB.Close()
			logger.Info("Successfully connected to PostgreSQL via GORM")
		}
	}

	if dbInitErr == nil {
		// Run Database Migrations Automatically
		logger.Info("Starting auto-migration check...")
		if err := migration.RunAutoMigrations(sqlDB); err != nil {
			logger.Error("Failed to run database migrations: %v", err)
			dbInitErr = fmt.Errorf("RunAutoMigrations: %w", err)
		}
	}

	if dbInitErr == nil {
		// GORM AutoMigrate Fallback
		logger.Info("Running GORM AutoMigrate as fallback...")
		err = db.AutoMigrate(
			&domain.Route{},
			&domain.Stop{},
			&domain.RouteStop{},
			&domain.Schedule{},
			&domain.Vehicle{},
			&domain.Report{},
		)
		if err != nil {
			logger.Error("GORM AutoMigrate failed: %v", err)
			dbInitErr = fmt.Errorf("AutoMigrate: %w", err)
		} else {
			logger.Info("GORM AutoMigrate completed successfully")
		}
	}

	// Initialize Repositories (Only if DB connected)
	var routeRepo domain.RouteRepository
	var reportRepo domain.ReportRepository
	var vehicleRepo domain.VehicleRepository
	var routeUsecase domain.RouteUsecase
	var reportUsecase domain.ReportUsecase

	if dbInitErr == nil {
		routeRepo = repository.NewRouteRepository(db)
		reportRepo = repository.NewReportRepository(db)
		vehicleRepo = repository.NewVehicleRepository(db)
		
		routeUsecase = usecase.NewRouteUsecase(routeRepo)
		reportUsecase = usecase.NewReportUsecase(reportRepo)
	}

	// Initialize Storage — pilih Azure Blob Storage jika connection string tersedia,
	// fallback ke LocalStorage untuk development lokal.
	var storageProvider storage.StorageProvider
	if cfg.AzureStorageConnectionString != "" {
		logger.Info("Menggunakan Azure Blob Storage (container: %s)", cfg.AzureStorageContainerName)
		azureStorage, err := storage.NewAzureBlobStorage(
			cfg.AzureStorageConnectionString,
			cfg.AzureStorageContainerName,
			cfg.AzureStorageAccountName,
		)
		if err != nil {
			logger.Fatal("Gagal inisialisasi Azure Blob Storage: %v", err)
		}
		storageProvider = azureStorage
	} else {
		logger.Info("AZURE_STORAGE_CONNECTION_STRING tidak di-set, menggunakan LocalStorage")
		ls, err := storage.NewLocalStorage("./uploads/reports", "/uploads/reports")
		if err != nil {
			logger.Fatal("Gagal inisialisasi storage: %v", err)
		}
		storageProvider = ls
	}

	// Context for background jobs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Background Jobs if DB initialized successfully
	if dbInitErr == nil {
		go reportUsecase.RunAutoResolve(ctx)
	}

	// Initialize WebSocket Server
	var wsServer *notification.WebSocketServer
	if dbInitErr == nil {
		wsServer = notification.NewWebSocketServer(vehicleRepo)
	}

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

	if wsServer != nil {
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
	}

	// Initialize Gin Engine
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Healthcheck endpoint (Unconditional 200 OK for Azure Container Apps)
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

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

	// Debug Endpoint for Database Migrations
	router.GET("/debug/db", func(c *gin.Context) {
		var err1Msg, err2Msg string
		
		if dbInitErr != nil {
			err1Msg = fmt.Sprintf("INIT ERROR: %v", dbInitErr)
			err2Msg = "Skipped due to init error"
		} else {
			err1 := migration.RunAutoMigrations(sqlDB)
			err2 := db.AutoMigrate(
				&domain.Route{},
				&domain.Stop{},
				&domain.RouteStop{},
				&domain.Schedule{},
				&domain.Vehicle{},
				&domain.Report{},
			)

			if err1 != nil {
				err1Msg = err1.Error()
			} else {
				err1Msg = "Success / Skipped"
			}
			
			if err2 != nil {
				err2Msg = err2.Error()
			} else {
				err2Msg = "Success"
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"0_Init_Status":        dbInitErr == nil,
			"1_AutoMigrations_SQL": err1Msg,
			"2_GORM_AutoMigrate":   err2Msg,
			"database_url_used":    cfg.DatabaseURL,
		})
	})

	// Setup API group
	api := router.Group("/api")

	if dbInitErr == nil {
		// Initialize HTTP Handlers
		delivery.NewRouteHandler(api, routeUsecase)
		delivery.NewReportHandler(api, reportUsecase, storageProvider)

		// Register WebSocket route
		wsServer.RegisterRoutes(router)

		// Mulai simulator kendaraan di background (goroutine)
		go wsServer.RunSimulator(ctx)
	} else {
		// Add a fallback endpoint to explain why API is down
		api.Any("/*path", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "API is currently unavailable due to a database initialization error. Check /debug/db for details.",
			})
		})
	}

	// Serve file gambar yang di-upload oleh user
	// URL: GET /uploads/reports/<filename.jpg>
	router.Static("/uploads", "./uploads")

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
