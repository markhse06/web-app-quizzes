package app

import (
	"Backend/internal/domain"
	"Backend/internal/dto"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user is not authenticated"})
		return uuid.Nil, false
	}

	userIDValue, ok := value.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authenticated user"})
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(userIDValue)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authenticated user"})
		return uuid.Nil, false
	}

	return userID, true
}

func quizIDFromParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quiz id"})
		return uuid.Nil, false
	}
	return id, true
}

func (a *App) handleCreateQuiz(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}

	var req dto.CreateQuizRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	quiz := domain.Quiz{
		CreatorID:        userID,
		Title:            req.Title,
		QuestionDuration: req.QuestionDuration,
	}

	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&quiz).Error; err != nil {
			return err
		}

		for _, questionRequest := range req.Questions {
			question := domain.Question{
				QuizID:  quiz.ID,
				Type:    questionRequest.Type,
				Content: questionRequest.Content,
			}
			if err := tx.Create(&question).Error; err != nil {
				return err
			}

			for _, answerRequest := range questionRequest.Answers {
				answer := domain.Answer{
					QuestionID: question.ID,
					Content:    answerRequest.Content,
					IsCorrect:  answerRequest.IsCorrect,
				}
				if err := tx.Create(&answer).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		a.logger.Error("failed to create quiz", "error", err, "creator_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create quiz"})
		return
	}

	if err := a.db.Preload("Questions.Answers").First(&quiz, quiz.ID).Error; err != nil {
		a.logger.Error("failed to load created quiz", "error", err, "quiz_id", quiz.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load created quiz"})
		return
	}
	c.JSON(http.StatusCreated, quiz)
}

func (a *App) handleListQuizzes(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}

	var quizzes []domain.Quiz
	if err := a.db.Where("creator_id = ?", userID).Order("created_at DESC").Find(&quizzes).Error; err != nil {
		a.logger.Error("failed to list quizzes", "error", err, "creator_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list quizzes"})
		return
	}
	c.JSON(http.StatusOK, quizzes)
}

func (a *App) handleGetQuiz(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	quizID, ok := quizIDFromParam(c)
	if !ok {
		return
	}

	var quiz domain.Quiz
	err := a.db.Where("id = ? AND creator_id = ?", quizID, userID).
		Preload("Questions.Answers").First(&quiz).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "quiz not found"})
		return
	}
	if err != nil {
		a.logger.Error("failed to get quiz", "error", err, "quiz_id", quizID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get quiz"})
		return
	}
	c.JSON(http.StatusOK, quiz)
}

func (a *App) handleDeleteQuiz(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	quizID, ok := quizIDFromParam(c)
	if !ok {
		return
	}

	result := a.db.Where("id = ? AND creator_id = ?", quizID, userID).Delete(&domain.Quiz{})
	if result.Error != nil {
		a.logger.Error("failed to delete quiz", "error", result.Error, "quiz_id", quizID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete quiz"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "quiz not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
