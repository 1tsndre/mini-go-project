package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
)

const userColumns = "id, email, password, name, role, created_at, updated_at"

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role string) error
}

type userRepository struct {
	db databases.Database
}

func NewUserRepository(db databases.Database) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	err := r.db.DB().QueryRowxContext(ctx, `
		INSERT INTO users (email, password, name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING `+userColumns,
		user.Email, user.Password, user.Name, user.Role,
	).StructScan(user)
	return translateError(err)
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.DB().GetContext(ctx, &user, "SELECT "+userColumns+" FROM users WHERE id = $1", id)
	if err != nil {
		return nil, translateError(err)
	}
	return &user, nil
}

// FindByEmail matches case-insensitively, which also finds accounts stored with
// mixed-case emails before registration started lower-casing them. If such an
// old account has a case variant, the older one wins.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.DB().GetContext(ctx, &user, `
		SELECT `+userColumns+`
		FROM users
		WHERE lower(email) = lower($1)
		ORDER BY created_at, id
		LIMIT 1`,
		email,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &user, nil
}

func (r *userRepository) UpdateRole(ctx context.Context, id uuid.UUID, role string) error {
	_, err := r.db.DB().ExecContext(ctx, "UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2", role, id)
	return translateError(err)
}
