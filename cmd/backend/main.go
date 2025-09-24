package main

import (
	"backend/internal/api"
	"log"
)

func main() {
	log.Println("App start")
	api.StartServer()
	log.Println("App terminated")
}
