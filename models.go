package media

import (
	"time"

	"github.com/google/uuid"
)

type Media struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	StorageKey    string     `json:"storage_key" db:"storage_key"`
	StorageDriver string     `json:"storage_driver" db:"storage_driver"`
	MimeType      string     `json:"mime_type" db:"mime_type"`
	Kind          string     `json:"kind" db:"kind"`
	Extension     string     `json:"extension" db:"extension"`
	Size          int64      `json:"size" db:"size"`
	Checksum      string     `json:"checksum" db:"checksum"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

func (Media) TableName() string { return "media" }
