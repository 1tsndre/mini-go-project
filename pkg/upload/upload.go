package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/constant"
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
		return "", fmt.Errorf(constant.MsgFileTooLarge, ErrInvalidFile, u.maxSize)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		return "", fmt.Errorf(constant.MsgExtensionNotAllowed, ErrInvalidFile, ext)
	}

	dir := filepath.Join(u.baseDir, subDir)
	if err := os.MkdirAll(dir, constant.UploadDirPerm); err != nil {
		return "", fmt.Errorf(constant.MsgCreateUploadDirFailed, err)
	}

	filename := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	filePath := filepath.Join(dir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf(constant.MsgCreateFileFailed, err)
	}

	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		os.Remove(filePath)
		return "", fmt.Errorf(constant.MsgWriteFileFailed, err)
	}

	if err := dst.Close(); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf(constant.MsgFinalizeFileFailed, err)
	}

	return filepath.Join(subDir, filename), nil
}

func (u *Uploader) Delete(path string) {
	if path == "" {
		return
	}
	os.Remove(filepath.Join(u.baseDir, path))
}
