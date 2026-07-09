package media

import (
	"time"

	"github.com/google/uuid"
)

type MediaUpdateDTO struct {
	Name string `json:"name"`
}

type MediaResponseDTO struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	MimeType  string     `json:"mime_type"`
	Kind      string     `json:"kind"`
	Extension string     `json:"extension"`
	Size      int64      `json:"size"`
	Checksum  string     `json:"checksum"`
	URL       string     `json:"url"`
	UserID    uuid.UUID  `json:"user_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}
