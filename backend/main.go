package main

import (
	"Backend/internal/app"
	"log"
)

func main() {
	application, err := app.NewApp()
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	application.Router().Static("/uploads", "./uploads")

	if err := application.Run(); err != nil {
		log.Fatalf("failed to run app: %v", err)
	}
}
