package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

type Uploader struct {
	baseDir string
	maxSize int64
}

func NewUploader(baseDir string, maxSize int64) *Uploader {
	return &Uploader{
		baseDir: baseDir,
		maxSize: maxSize,
	}
}

func (u *Uploader) Upload(file multipart.File, header *multipart.FileHeader, subDir string) (string, error) {
	if header.Size > u.maxSize {
		return "", fmt.Errorf("file size exceeds maximum allowed size of %d bytes", u.maxSize)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		return "", fmt.Errorf("file extension %s is not allowed", ext)
	}

	dir := filepath.Join(u.baseDir, subDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	filePath := filepath.Join(dir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		os.Remove(filePath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if err := dst.Close(); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to finalize file: %w", err)
	}

	return filepath.Join(subDir, filename), nil
}

func (u *Uploader) Delete(relativePath string) error {
	fullPath := filepath.Join(u.baseDir, relativePath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
