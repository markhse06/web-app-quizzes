package app

import (
	"Backend/internal/config"
	"Backend/internal/db"
	"io"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

type App struct {
	router *gin.Engine
	db     *db.DB
	logger *slog.Logger
}

func NewApp() (*App, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, err
	}

	r := gin.Default()

	database, err := db.NewDB()
	if err != nil {
		return nil, err
	}

	a := &App{
		router: r,
		db:     database,
		logger: logger,
	}

	a.registerRoutes()

	return a, nil
}

func (a *App) Run() error {
	cfg := config.LoadHTTP()
	return a.router.Run(":" + cfg.Port)
}

func (a *App) Router() *gin.Engine {
	return a.router
}

func NewLogger() (*slog.Logger, error) {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	writer := io.MultiWriter(os.Stdout, file)
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	return logger, nil
}
