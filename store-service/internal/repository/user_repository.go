package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const userColumns = "id, email, password, name, role, created_at, updated_at"

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO users (email, password, name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING `+userColumns,
		user.Email, user.Password, user.Name, user.Role,
	).StructScan(user)
	return translateDatabaseError(err)
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User

	err := r.db.GetContext(ctx, &user, "SELECT "+userColumns+" FROM users WHERE id = $1", id)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User

	err := r.db.GetContext(ctx, &user, "SELECT "+userColumns+" FROM users WHERE email = $1", email)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	return &user, nil
}
