package auth

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTProvider struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTProvider(secret string, ttl time.Duration) *JWTProvider {
	return &JWTProvider{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (p *JWTProvider) CreateGuestToken(_ context.Context, actorID, displayName string) (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"sub":         actorID,
		"actor_type":  "guest",
		"displayName": displayName,
		"iat":         now.Unix(),
		"exp":         now.Add(p.ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secret)
}
