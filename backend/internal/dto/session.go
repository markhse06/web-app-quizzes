package dto

type CreateSessionRequest struct {
	QuizID string `json:"quiz_id" binding:"required,uuid4"`
}
