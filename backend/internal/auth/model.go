package auth

import (
	"context"
	"time"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name"`
	Role          string     `json:"role"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	VerifiedAt    *time.Time `json:"-"`
}

type identityContextKey struct{}

func WithIdentity(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, identityContextKey{}, user)
}

func IdentityFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(identityContextKey{}).(User)
	return user, ok
}

func IsAdmin(ctx context.Context) bool {
	user, ok := IdentityFromContext(ctx)
	return ok && user.Role == RoleAdmin
}
