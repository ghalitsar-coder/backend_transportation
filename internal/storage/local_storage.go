package storage

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────
//  StorageProvider interface
//
//  Interface ini memudahkan migrasi storage di masa depan.
//  Cukup buat implementasi baru (misal: AzureBlobStorage)
//  dan inject ke usecase tanpa mengubah business logic.
// ─────────────────────────────────────────────────────────────

// StorageProvider mendefinisikan kontrak penyimpanan file.
type StorageProvider interface {
	// Save menyimpan file dari multipart upload dan mengembalikan URL yang
	// bisa diakses oleh browser (path relatif atau URL publik ke cloud).
	Save(file multipart.File, header *multipart.FileHeader) (url string, err error)
}

// ─────────────────────────────────────────────────────────────
//  LocalStorage — implementasi untuk development lokal
// ─────────────────────────────────────────────────────────────

// LocalStorage menyimpan file ke direktori lokal di filesystem.
// URL yang dikembalikan adalah path relatif yang di-serve oleh Gin Static().
type LocalStorage struct {
	// BaseDir: direktori tempat file disimpan, misal "./uploads/reports"
	BaseDir string
	// BaseURL: prefix URL untuk mengakses file, misal "/uploads/reports"
	BaseURL string
}

// NewLocalStorage membuat instance LocalStorage dan memastikan direktori ada.
func NewLocalStorage(baseDir, baseURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("gagal membuat direktori storage: %w", err)
	}
	return &LocalStorage{BaseDir: baseDir, BaseURL: baseURL}, nil
}

// Save menyimpan file multipart ke disk.
// - File di-rename dengan UUID untuk mencegah collision
// - Ekstensi asli dipertahankan
// - Hanya file gambar yang diizinkan (validasi MIME type)
func (s *LocalStorage) Save(file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validasi MIME type — hanya izinkan gambar
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".gif":  true,
	}
	if !allowedExts[ext] {
		return "", fmt.Errorf("tipe file tidak diizinkan: %s (gunakan jpg, png, webp, gif)", ext)
	}

	// Generate nama file unik dengan UUID
	fileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	filePath := filepath.Join(s.BaseDir, fileName)

	// Buat file tujuan
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("gagal membuat file: %w", err)
	}
	defer dst.Close()

	// Copy isi file
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := file.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("gagal menulis file: %w", werr)
			}
		}
		if err != nil {
			break
		}
	}

	// Return URL yang bisa diakses browser
	url := fmt.Sprintf("%s/%s", strings.TrimRight(s.BaseURL, "/"), fileName)
	return url, nil
}
