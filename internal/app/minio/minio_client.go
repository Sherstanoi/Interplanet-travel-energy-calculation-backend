package minio

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client
var BucketName = "media"

func InitMinio() {
	endpoint := "localhost:9000"
	accessKeyID := "root"
	secretAccessKey := "rootpassword"
	useSSL := false

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln("Ошибка подключения к Minio:", err)
	}

	MinioClient = minioClient
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, BucketName)
	if err != nil {
		log.Fatalln("Ошибка проверки бакета:", err)
	}
	if !exists {
		if err := minioClient.MakeBucket(ctx, BucketName, minio.MakeBucketOptions{}); err != nil {
			log.Fatalln("Ошибка создания бакета:", err)
		}
	}
	log.Println("Minio client initialized")
}

func GetPhotoURL(objectName string) (string, error) {
	if MinioClient == nil {
		return "", fmt.Errorf("Minio client не инициализирован")
	}

	ctx := context.Background()

	expiry := 24 * time.Hour

	presignedURL, err := MinioClient.PresignedGetObject(
		ctx,
		BucketName,
		objectName,
		expiry,
		nil,
	)
	if err != nil {
		log.Println("Ошибка получения presigned URL:", err)
		return "", err
	}

	return presignedURL.String(), nil
}

// UploadObject кладет файл в бакет под сгенерированным именем.
func UploadObject(ctx context.Context, objectName string, r io.Reader, size int64, contentType string) error {
	if MinioClient == nil {
		return fmt.Errorf("Minio client не инициализирован")
	}
	_, err := MinioClient.PutObject(ctx, BucketName, objectName, r, size,
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

// RemoveObject нужен для отката, если запись в БД не удалась.
func RemoveObject(ctx context.Context, objectName string) error {
	if MinioClient == nil {
		return fmt.Errorf("Minio client не инициализирован")
	}
	return MinioClient.RemoveObject(ctx, BucketName, objectName, minio.RemoveObjectOptions{})
}
