package model

import (
	"time"

	"github.com/google/uuid"
)

type Store struct {
	ID          uuid.UUID `db:"id" json:"id"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	LogoURL     string    `db:"logo_url" json:"logo_url"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
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
