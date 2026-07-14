package app

import (
	"Backend/internal/domain"
	"Backend/internal/dto"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var bigSixDigitRange = big.NewInt(900000)

func (a *App) handleCreateSession(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}

	var req dto.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	quizID, err := uuid.Parse(req.QuizID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quiz id"})
		return
	}

	var quiz domain.Quiz
	if err := a.db.Where("id = ? AND creator_id = ?", quizID, userID).First(&quiz).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quiz not found"})
		return
	}

	for range 10 {
		pin, err := generatePIN()
		if err != nil {
			a.logger.Error("failed to generate session pin", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
			return
		}

		session := domain.GameSession{QuizID: quiz.ID, PinCode: pin, Status: "waiting"}
		if err := a.db.Create(&session).Error; err == nil {
			a.hub.CreateRoom(session)
			c.JSON(http.StatusCreated, gin.H{"session_id": session.ID, "pin_code": session.PinCode, "status": session.Status})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate a unique pin"})
}

func generatePIN() (string, error) {
	value, err := rand.Int(rand.Reader, bigSixDigitRange)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()+100000), nil
}
