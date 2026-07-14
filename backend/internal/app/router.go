package app

import "github.com/gin-gonic/gin"

func (a *App) registerRoutes() {
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	api := a.router.Group("/api")
	{
		user := api.Group("/user")
		{
			user.POST("/register", a.handleUserRegistration)
			user.POST("/login", a.handleUserLogin)
			user.POST("refresh_token", a.handleRefreshToken)
		}

		quizzes := api.Group("/quizzes")
		quizzes.Use(a.AuthMiddleware())
		{
			quizzes.POST("", a.handleCreateQuiz)
			quizzes.GET("", a.handleListQuizzes)
			quizzes.GET("/:id", a.handleGetQuiz)
			quizzes.DELETE("/:id", a.handleDeleteQuiz)
		}

		api.POST("/upload", a.AuthMiddleware(), a.handleUpload)
	}
}
