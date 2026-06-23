package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrEmailTaken         = errors.New("email already registered")
)

const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

// Claims is the JWT payload embedded in every access token.
type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
}

// TokenPair holds an issued access token and its companion refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// AuthService handles user registration, login, token issuance and rotation.
type AuthService struct {
	users  internal.UserRepo
	tokens internal.RefreshTokenRepo
	secret []byte
}

// NewAuthService creates an AuthService backed by the provided repos and HMAC secret.
func NewAuthService(users internal.UserRepo, tokens internal.RefreshTokenRepo, secret string) *AuthService {
	return &AuthService{users: users, tokens: tokens, secret: []byte(secret)}
}

// Register creates a new user with an argon2id-hashed password.
func (s *AuthService) Register(ctx context.Context, email, password string) (*models.User, error) {
	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return nil, err
	}
	user := &models.User{Email: email, PasswordHash: hash}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login verifies credentials and issues an access + refresh token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, *TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrInvalidCredentials
	}
	match, err := argon2id.ComparePasswordAndHash(password, user.PasswordHash)
	if err != nil || !match {
		return nil, nil, ErrInvalidCredentials
	}
	pair, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// Refresh validates a refresh token, revokes it, and issues a new token pair.
func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*TokenPair, error) {
	hash := hashToken(rawToken)
	tok, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if tok == nil || tok.RevokedAt != nil || time.Now().After(tok.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	if err := s.tokens.Revoke(ctx, tok.ID); err != nil {
		return nil, err
	}
	return s.issueTokenPair(ctx, tok.UserID)
}

// Logout revokes the refresh token for rawToken (noop if not found or already revoked).
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	hash := hashToken(rawToken)
	tok, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return err
	}
	if tok == nil || tok.RevokedAt != nil {
		return nil
	}
	return s.tokens.Revoke(ctx, tok.ID)
}

// GetUser returns the user with the given ID.
func (s *AuthService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return s.users.FindByID(ctx, userID)
}

// ValidateAccessToken parses and validates a JWT access token, returning the userID.
func (s *AuthService) ValidateAccessToken(tokenStr string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return "", ErrInvalidToken
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || claims.UserID == "" {
		return "", ErrInvalidToken
	}
	return claims.UserID, nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID string) (*TokenPair, error) {
	accessToken, err := s.signAccessToken(userID)
	if err != nil {
		return nil, err
	}
	rawRefresh, err := generateRawToken()
	if err != nil {
		return nil, err
	}
	tok := &models.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(rawRefresh),
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	if err := s.tokens.Create(ctx, tok); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    time.Now().Add(accessTokenTTL),
	}, nil
}

func (s *AuthService) signAccessToken(userID string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
		UserID: userID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
