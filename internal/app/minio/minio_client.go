package minio

import (
	"context"
	"fmt"
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
	log.Println("Minio client initialized")
}

func GetImageURL(objectName string) (string, error) {
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
