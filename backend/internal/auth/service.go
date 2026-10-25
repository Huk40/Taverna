package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrEmailDeliveryUnavailable = errors.New("email delivery is not configured")
	ErrInvalidInput             = errors.New("invalid input")
)

type Config struct {
	SessionTTL time.Duration
	Mailer     EmailSender
}
type EmailSender interface {
	Configured() bool
	SendVerification(context.Context, string, string) error
	SendPasswordReset(context.Context, string, string) error
}
type Service struct {
	repo       *Repository
	sessionTTL time.Duration
	mailer     EmailSender
}

func NewService(repo *Repository, cfg Config) *Service {
	if cfg.Mailer == nil {
		cfg.Mailer = Mailer{}
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 30 * 24 * time.Hour
	}
	return &Service{repo: repo, sessionTTL: cfg.SessionTTL, mailer: cfg.Mailer}
}

func (s *Service) Register(ctx context.Context, email, name, password string) (User, string, bool, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, "", false, fmt.Errorf("%w: укажите корректный email", ErrInvalidInput)
	}
	if err := validatePassword(password); err != nil {
		return User{}, "", false, err
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 80 {
		return User{}, "", false, fmt.Errorf("%w: имя не должно превышать 80 символов", ErrInvalidInput)
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, "", false, err
	}
	// Public registration never grants privileges or claims existing projects.
	user, err := s.repo.CreateUser(ctx, email, name, hash, RoleUser)
	if err != nil {
		return User{}, "", false, err
	}
	token, err := s.createSession(ctx, user.ID, hash)
	if err != nil {
		return User{}, "", false, err
	}
	sent := s.ResendVerification(ctx, user) == nil
	return user, token, sent, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) > 256 {
		return User{}, "", ErrInvalidCredentials
	}
	credentials, err := s.repo.CredentialsByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		_ = verifyPassword(dummyPasswordHash, password)
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	if !verifyPassword(credentials.PasswordHash, password) {
		return User{}, "", ErrInvalidCredentials
	}
	token, err := s.createSession(ctx, credentials.ID, credentials.PasswordHash)
	return credentials.User, token, err
}

func (s *Service) UserForSession(ctx context.Context, token string) (User, error) {
	if !validToken(token) {
		return User{}, ErrUserNotFound
	}
	return s.repo.UserBySession(ctx, sessionTokenHash(token))
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if !validToken(token) {
		return nil
	}
	return s.repo.DeleteSession(ctx, sessionTokenHash(token))
}

func (s *Service) StartPasswordReset(ctx context.Context, email string) error {
	if !s.mailer.Configured() {
		return ErrEmailDeliveryUnavailable
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return nil
	}
	credentials, err := s.repo.CredentialsByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	token, err := s.issueToken(ctx, credentials.ID, "reset_password", time.Hour)
	if err != nil {
		return err
	}
	return s.mailer.SendPasswordReset(ctx, credentials.Email, token)
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if !validToken(token) {
		return ErrInvalidEmailToken
	}
	_, err := s.repo.VerifyEmailToken(ctx, sessionTokenHash(token))
	return err
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if !validToken(token) {
		return ErrInvalidEmailToken
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.repo.ResetPasswordFromToken(ctx, sessionTokenHash(token), hash)
	return err
}

func (s *Service) EmailDeliveryConfigured() bool { return s.mailer.Configured() }
func (s *Service) SessionTTL() time.Duration     { return s.sessionTTL }
func (s *Service) ResendVerification(ctx context.Context, user User) error {
	if user.EmailVerified {
		return nil
	}
	if !s.mailer.Configured() {
		return ErrEmailDeliveryUnavailable
	}
	token, err := s.issueToken(ctx, user.ID, "verify_email", 24*time.Hour)
	if err != nil {
		return err
	}
	return s.mailer.SendVerification(ctx, user.Email, token)
}

func (s *Service) issueToken(ctx context.Context, userID, kind string, ttl time.Duration) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := s.repo.IssueEmailToken(ctx, userID, kind, sessionTokenHash(token), time.Now().UTC().Add(ttl)); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) createSession(ctx context.Context, userID, passwordHash string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	err = s.repo.CreateSession(ctx, userID, passwordHash, sessionTokenHash(token), time.Now().UTC().Add(s.sessionTTL))
	return token, err
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func validToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32 && len(token) == 43
}
func normalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalidInput
	}
	return strings.ToLower(value), nil
}
func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < 10 {
		return fmt.Errorf("%w: пароль должен содержать не менее 10 символов", ErrInvalidInput)
	}
	if len(password) > 256 {
		return fmt.Errorf("%w: пароль слишком длинный", ErrInvalidInput)
	}
	return nil
}
