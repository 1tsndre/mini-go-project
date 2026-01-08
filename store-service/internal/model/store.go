package model

import (
	"time"

	"github.com/google/uuid"
)

type Store struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	LogoURL     string    `json:"logo_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	User     User      `gorm:"foreignKey:UserID" json:"-"`
	Products []Product `gorm:"foreignKey:StoreID" json:"-"`
}

type CreateStoreRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateStoreRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type StoreResponse struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	LogoURL     string    `json:"logo_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *Store) ToResponse() StoreResponse {
	return StoreResponse{
		ID:          s.ID,
		UserID:      s.UserID,
		Name:        s.Name,
		Description: s.Description,
		LogoURL:     s.LogoURL,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
