package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

type MinIOClient struct {
	client     *minio.Client
	bucketName string
}

// NewMinIOClient создает клиент для MinIO
func NewMinIOClient(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// Создаем bucket если не существует
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logrus.Infof("Bucket %s created successfully", bucketName)
	}

	return &MinIOClient{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// UploadFile загружает файл в MinIO и возвращает имя файла
func (m *MinIOClient) UploadFile(fileData []byte, originalFilename string) (string, error) {
	ctx := context.Background()

	// Генерируем уникальное имя файла на латинице
	ext := filepath.Ext(originalFilename)
	newFilename := fmt.Sprintf("service_%s_%d%s",
		uuid.New().String()[:8],
		time.Now().Unix(),
		ext)

	// Определяем content type
	contentType := "application/octet-stream"
	extLower := strings.ToLower(ext)
	switch extLower {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	}

	// Загружаем файл
	reader := bytes.NewReader(fileData)
	_, err := m.client.PutObject(ctx, m.bucketName, newFilename, reader, int64(len(fileData)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	logrus.Infof("File %s uploaded successfully", newFilename)
	return newFilename, nil
}

// DeleteFile удаляет файл из MinIO
func (m *MinIOClient) DeleteFile(filename string) error {
	ctx := context.Background()

	err := m.client.RemoveObject(ctx, m.bucketName, filename, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	logrus.Infof("File %s deleted successfully", filename)
	return nil
}

// GetFileURL возвращает временный URL для доступа к файлу (1 час)
func (m *MinIOClient) GetFileURL(filename string) (string, error) {
	ctx := context.Background()

	url, err := m.client.PresignedGetObject(ctx, m.bucketName, filename, time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url.String(), nil
}

// DownloadFile скачивает файл из MinIO
func (m *MinIOClient) DownloadFile(filename string) ([]byte, error) {
	ctx := context.Background()

	object, err := m.client.GetObject(ctx, m.bucketName, filename, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return data, nil
}

// FileExists проверяет существует ли файл
func (m *MinIOClient) FileExists(filename string) (bool, error) {
	ctx := context.Background()

	_, err := m.client.StatObject(ctx, m.bucketName, filename, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}
