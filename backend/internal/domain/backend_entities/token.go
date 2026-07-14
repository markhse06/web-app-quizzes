package backend_entities

import (
	"time"

	"github.com/google/uuid"
)

type Token struct {
	ID        uuid.UUID `gorm:"primary_key;type:uuid;default:uuid_generate_v4()"`
	UserID    uuid.UUID `gorm:"primary_key"`
	Secret    string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"type:timestamp;default:current_timestamp"`
	ExpiresAt time.Time `gorm:"type:timestamp"`
}
