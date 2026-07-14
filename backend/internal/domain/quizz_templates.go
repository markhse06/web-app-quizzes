package domain

import (
	"time"

	"github.com/google/uuid"
)

// User — организатор квиза
type User struct {
	ID           uuid.UUID `gorm:"primary_key;type:uuid;default:uuid_generate_v4()"`
	Email        string    `gorm:"unique;not null"`
	PasswordHash string    `gorm:"not null"`
	Quizzes      []Quiz    `gorm:"foreignKey:CreatorID"`
	CreatedAt    time.Time `gorm:"not null;default:now()"`
	UpdatedAt    time.Time
}

// Quiz - шаблон квиза
type Quiz struct {
	ID               uuid.UUID     `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()"`
	CreatorID        uuid.UUID     `gorm:"not null"`
	Title            string        `gorm:"not null"`
	QuestionDuration int           `gorm:"not null"`
	Questions        []Question    `gorm:"foreignKey:QuizID;constraint:OnDelete:CASCADE;"`
	GameSessions     []GameSession `gorm:"foreignKey:QuizID"`
	CreatedAt        time.Time     `gorm:"not null;default:now()"`
}

// Question — вопрос внутри квиза
type Question struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()"`
	QuizID    uuid.UUID `gorm:"not null"`
	Type      string    `gorm:"not null"` // "single", "multiple"
	Content   string    `gorm:"not null"` // Текст вопроса
	ImagePath string
	Answers   []Answer `gorm:"foreignKey:QuestionID;constraint:OnDelete:CASCADE;"`
}

// Answer — вариант ответа созданный организатором
type Answer struct {
	ID         uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()"`
	QuestionID uuid.UUID `gorm:"not null"`
	Content    string    `gorm:"not null"`
	IsCorrect  bool      `gorm:"not null;default:false"`
}
