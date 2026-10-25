package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidEmailToken      = errors.New("invalid or expired email token")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type userCredentials struct {
	User
	PasswordHash string
}

func (r *Repository) CreateUser(ctx context.Context, email, displayName, passwordHash, role string) (User, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, email, display_name, role, email_verified_at, created_at, updated_at
	`, email, displayName, passwordHash, role)
	user, err := scanUser(row)
	if err != nil && isUniqueViolation(err) {
		return User{}, ErrEmailAlreadyRegistered
	}
	return user, err
}

func (r *Repository) CredentialsByEmail(ctx context.Context, email string) (userCredentials, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, role, email_verified_at, created_at, updated_at, password_hash
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`, email)
	var result userCredentials
	if err := scanUserWithPassword(row, &result); err != nil {
		return userCredentials{}, err
	}
	return result, nil
}

func (r *Repository) CreateSession(ctx context.Context, userID, passwordHash string, tokenHash []byte, expiresAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO user_sessions (user_id, token_hash, expires_at)
  SELECT id, $2, $3 FROM users WHERE id=$1 AND password_hash=$4 FOR SHARE
	`, userID, tokenHash, expiresAt, passwordHash)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrInvalidCredentials
	}
	return err
}

func (r *Repository) DeleteAllSessions(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_sessions WHERE user_id=$1`, userID)
	return err
}

func (r *Repository) UserBySession(ctx context.Context, tokenHash []byte) (User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.display_name, u.role, u.email_verified_at, u.created_at, u.updated_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW() AND s.last_seen_at > NOW() - INTERVAL '7 days'
	`, tokenHash)
	user, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE user_sessions SET last_seen_at = NOW() WHERE token_hash = $1 AND last_seen_at < NOW() - INTERVAL '5 minutes'`, tokenHash)
	return user, nil
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *Repository) IssueEmailToken(ctx context.Context, userID, kind string, tokenHash []byte, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize issuance per user, including concurrent resends.
	if _, err = tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM user_email_tokens WHERE user_id = $1 AND kind = $2 AND used_at IS NULL`, userID, kind); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO user_email_tokens (user_id, kind, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, kind, tokenHash, expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) VerifyEmailToken(ctx context.Context, tokenHash []byte) (User, error) {
	return r.consumeToken(ctx, tokenHash, "verify_email", "")
}

func (r *Repository) ResetPasswordFromToken(ctx context.Context, tokenHash []byte, passwordHash string) (User, error) {
	return r.consumeToken(ctx, tokenHash, "reset_password", passwordHash)
}

func (r *Repository) consumeToken(ctx context.Context, tokenHash []byte, kind, newPasswordHash string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	if err := tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM user_email_tokens
		WHERE token_hash = $1 AND kind = $2 AND used_at IS NULL AND expires_at > NOW()
	`, tokenHash, kind).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidEmailToken
		}
		return User{}, err
	}

	if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
		return User{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE user_email_tokens SET used_at=NOW() WHERE token_hash=$1 AND used_at IS NULL AND expires_at>NOW()`, tokenHash)
	if err != nil {
		return User{}, err
	}
	if tag.RowsAffected() != 1 {
		return User{}, ErrInvalidEmailToken
	}
	if _, err := tx.Exec(ctx, `UPDATE user_email_tokens SET used_at = NOW() WHERE user_id=$1 AND kind=$2 AND used_at IS NULL`, userID, kind); err != nil {
		return User{}, err
	}
	if kind == "verify_email" {
		if _, err := tx.Exec(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, NOW()), updated_at = NOW() WHERE id = $1`, userID); err != nil {
			return User{}, err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2, email_verified_at=COALESCE(email_verified_at,NOW()), updated_at = NOW() WHERE id = $1`, userID, newPasswordHash); err != nil {
			return User{}, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM user_sessions WHERE user_id = $1`, userID); err != nil {
			return User{}, err
		}
	}

	row := tx.QueryRow(ctx, `
		SELECT id::text, email, display_name, role, email_verified_at, created_at, updated_at
		FROM users WHERE id = $1
	`, userID)
	user, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func sessionTokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type userScanner interface{ Scan(dest ...any) error }

func scanUser(scanner userScanner) (User, error) {
	var user User
	var verifiedAt *time.Time
	if err := scanner.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &verifiedAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user.VerifiedAt = verifiedAt
	user.EmailVerified = verifiedAt != nil
	return user, nil
}

func scanUserWithPassword(scanner userScanner, result *userCredentials) error {
	var verifiedAt *time.Time
	if err := scanner.Scan(&result.ID, &result.Email, &result.DisplayName, &result.Role, &verifiedAt, &result.CreatedAt, &result.UpdatedAt, &result.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("scan user credentials: %w", err)
	}
	result.Email = strings.ToLower(strings.TrimSpace(result.Email))
	result.VerifiedAt = verifiedAt
	result.EmailVerified = verifiedAt != nil
	return nil
}
