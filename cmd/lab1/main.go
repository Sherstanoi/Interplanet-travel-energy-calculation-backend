package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	_ "lab1/docs" // сгенерированная командой swag init документация Swagger
	"lab1/internal/app/auth"
	"lab1/internal/app/config"
	"lab1/internal/app/dsn"
	"lab1/internal/app/handler"
	"lab1/internal/app/minio"
	appredis "lab1/internal/app/redis"
	"lab1/internal/app/repository"
	"lab1/internal/pkg"
)

// @title           Interplanet travel energy API
// @version         1.0
// @description     Бэкенд расчёта энергии межпланетных перелётов.
// @description     Без авторизации доступно только чтение (список и лента услуг). Методы создателя требуют JWT:
// @description     получите токен в POST /api/auth/login, нажмите Authorize и вставьте «Bearer <access_token>».

// @BasePath  /
// @schemes   http https

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 JWT в формате: Bearer <access_token>

func main() {
	ctx := context.Background()

	minio.InitMinio()
	router := gin.Default()

	conf, err := config.NewConfig()
	if err != nil {
		logrus.Fatalf("error loading config: %v", err)
	}

	rep, err := repository.NewRepository(dsn.FromEnv())
	if err != nil {
		logrus.Fatalf("error initializing repository: %v", err)
	}

	redisClient, err := appredis.New(ctx, conf.Redis)
	if err != nil {
		logrus.Fatalf("error initializing redis: %v", err)
	}
	defer redisClient.Close()

	tokens := auth.NewTokenManager(conf.JWT.Secret, conf.JWT.ExpiresIn)

	hand := handler.NewHandler(rep, redisClient, tokens)

	application := pkg.NewApp(conf, router, hand)
	application.RunApp()
}
