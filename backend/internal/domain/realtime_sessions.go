package domain

import (
	"time"

	"github.com/google/uuid"
)

// GameSession — Живая комната квиза
type GameSession struct {
	ID           uuid.UUID     `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()"`
	QuizID       uuid.UUID     `gorm:"not null"`
	PinCode      string        `gorm:"unique;not null"`
	Status       string        `gorm:"not null;default:'waiting'"` // "waiting", "playing", "finished"
	Participants []Participant `gorm:"foreignKey:SessionID"`
	CreatedAt    time.Time     `gorm:"not null;default:now()"`
}

// Participant — Игрок, подключившийся по PIN-коду
type Participant struct {
	ID            uuid.UUID      `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()"`
	SessionID     uuid.UUID      `gorm:"not null"`
	Name          string         `gorm:"not null"`
	Score         int            `gorm:"not null;default:0"`
	PlayerAnswers []PlayerAnswer `gorm:"foreignKey:ParticipantID"`
}

// PlayerAnswer — Что именно нажал пользователь в процессе игры
type PlayerAnswer struct {
	ID            uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()"`
	ParticipantID uuid.UUID `gorm:"not null"`
	QuestionID    uuid.UUID `gorm:"not null"`
	AnswerID      uuid.UUID `gorm:"not null"` // Ссылка на выбранный Answer.ID
}
