package api

import (
	"lab1/internal/app/handler"
	"lab1/internal/app/minio"
	"lab1/internal/app/repository"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func StartServer() {
	log.Println("Starting server")

	minio.InitMinio()

	repo, err := repository.NewRepository()
	if err != nil {
		logrus.Error("ошибка инициализации репозитория")
		return
	}

	handler := handler.NewHandler(repo)

	r := gin.Default()
	r.LoadHTMLGlob("../../templates/*")
	r.Static("/static", "../../resources")

	r.GET("/cards", handler.GetCards)
	r.GET("/vibes/:id", handler.GetCard)
	r.GET("/create", handler.GetDraft)

	r.Run()
	log.Println("Server down")
}
