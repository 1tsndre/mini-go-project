package model

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	ProductID uuid.UUID `db:"product_id" json:"product_id"`
	Rating    int       `db:"rating" json:"rating"`
	Comment   string    `db:"comment" json:"comment"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// UserName is the reviewer's name, joined from users when reviews are listed.
	UserName string `db:"user_name" json:"-"`
}

type CreateReviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

type ReviewResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	ProductID uuid.UUID `json:"product_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Review) ToResponse() ReviewResponse {
	return ReviewResponse{
		ID:        r.ID,
		UserID:    r.UserID,
		UserName:  r.UserName,
		ProductID: r.ProductID,
		Rating:    r.Rating,
		Comment:   r.Comment,
		CreatedAt: r.CreatedAt,
	}
}
