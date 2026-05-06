package storage

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────
//  AzureBlobStorage — implementasi cloud storage dengan Azure Blob
//
//  Implementasi ini memenuhi interface StorageProvider sehingga
//  bisa di-swap dengan LocalStorage tanpa mengubah business logic.
//  File gambar di-upload ke Azure Blob Container dan dikembalikan
//  sebagai URL publik yang dapat diakses langsung oleh browser.
// ─────────────────────────────────────────────────────────────

// AzureBlobStorage menyimpan file ke Azure Blob Storage.
type AzureBlobStorage struct {
	client        *azblob.Client
	containerName string
	baseURL       string // URL publik container, misal: https://<account>.blob.core.windows.net/<container>
}

// NewAzureBlobStorage membuat instance AzureBlobStorage dari connection string.
// containerName: nama blob container (misal "blobacacontainer")
// accountName  : nama storage account (misal "blobacaghal") — dipakai untuk membangun public URL
func NewAzureBlobStorage(connectionString, containerName, accountName string) (*AzureBlobStorage, error) {
	client, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat Azure Blob client: %w", err)
	}

	// Pastikan container ada (buat jika belum ada)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = client.CreateContainer(ctx, containerName, nil)
	if err != nil {
		// Abaikan error jika container sudah ada (kode 409 Conflict)
		// SDK mengembalikan *azcore.ResponseError dengan kode "ContainerAlreadyExists"
		if !strings.Contains(err.Error(), "ContainerAlreadyExists") {
			return nil, fmt.Errorf("gagal membuat container '%s': %w", containerName, err)
		}
	}

	baseURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName, containerName)

	return &AzureBlobStorage{
		client:        client,
		containerName: containerName,
		baseURL:       baseURL,
	}, nil
}

// Save mengupload file multipart ke Azure Blob Storage.
// Mengembalikan URL publik permanen ke blob yang diupload.
func (s *AzureBlobStorage) Save(file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validasi ekstensi file — hanya gambar yang diizinkan
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

	// Tentukan Content-Type berdasarkan ekstensi
	contentTypeMap := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
		".gif":  "image/gif",
	}
	contentType := contentTypeMap[ext]

	// Generate nama blob unik dengan UUID agar tidak terjadi konflik
	blobName := fmt.Sprintf("reports/%s%s", uuid.New().String(), ext)

	// Upload ke Azure
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uploadOptions := &azblob.UploadStreamOptions{
		BlockSize:   4 * 1024 * 1024, // 4MB per block
		Concurrency: 3,
		HTTPHeaders: &blob.HTTPHeaders{
			BlobContentType: &contentType,
		},
	}

	_, err := s.client.UploadStream(ctx, s.containerName, blobName, file, uploadOptions)
	if err != nil {
		return "", fmt.Errorf("gagal mengupload ke Azure Blob: %w", err)
	}

	// Return URL publik permanen ke blob
	publicURL := fmt.Sprintf("%s/%s", s.baseURL, blobName)
	return publicURL, nil
}
