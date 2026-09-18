package main

import (
	"lab1/internal/api"
	"log"
)

func main() {
	log.Println("Application start!")
	api.StartServer()
	log.Println("Applicxation Terminated!")
}
