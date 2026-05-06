package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	"transit-app/internal/logger"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

// RunAutoMigrations mengeksekusi semua file SQL migrasi secara berurutan
// jika tabel 'schema_migrations' belum mencatatnya.
func RunAutoMigrations(db *sql.DB) error {
	// Buat tabel tracking migrasi jika belum ada
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("gagal membuat tabel schema_migrations: %w", err)
	}

	// Baca semua file di direktori embed
	entries, err := migrationFiles.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("gagal membaca direktori embed sql: %w", err)
	}

	// Filter file *.up.sql dan urutkan
	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	// Eksekusi setiap file
	for _, file := range upFiles {
		version := strings.Split(file, "_")[0] // contoh "000001"

		// Cek apakah sudah diaplikasikan
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("gagal mengecek versi migrasi %s: %w", version, err)
		}

		if exists {
			continue // Sudah diaplikasikan, lewati
		}

		logger.Info("Menjalankan migrasi database: %s", file)
		content, err := migrationFiles.ReadFile("sql/" + file)
		if err != nil {
			return fmt.Errorf("gagal membaca file migrasi %s: %w", file, err)
		}

		// Jalankan di dalam transaksi
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("gagal memulai transaksi untuk migrasi: %w", err)
		}

		_, err = tx.Exec(string(content))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("gagal mengeksekusi file %s: %w", file, err)
		}

		_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("gagal mencatat versi migrasi %s: %w", version, err)
		}

		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("gagal commit migrasi %s: %w", file, err)
		}
		logger.Info("Migrasi %s berhasil diaplikasikan", file)
	}

	// Cek apakah seed.sql perlu dijalankan (jika tabel routes kosong)
	var routeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM routes").Scan(&routeCount)
	if err == nil && routeCount == 0 {
		logger.Info("Tabel routes kosong, menjalankan seed.sql...")
		seedContent, err := migrationFiles.ReadFile("sql/seed.sql")
		if err == nil {
			_, err = db.Exec(string(seedContent))
			if err != nil {
				logger.Error("Gagal menjalankan seed.sql: %v", err)
			} else {
				logger.Info("Seed database berhasil!")
			}
		}
	}

	return nil
}
