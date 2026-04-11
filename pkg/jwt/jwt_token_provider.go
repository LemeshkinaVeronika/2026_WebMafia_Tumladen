package jwtprovider

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTProvider struct {
	secret []byte
	ttl    time.Duration
}

type Claims struct {
	SessionID   string `json:"sessionId"`
	ActorID     string `json:"actorId"`
	ActorType   string `json:"actor_type"`
	DisplayName string `json:"displayName"`
	jwt.RegisteredClaims
}

func New(secret string, ttl time.Duration) *JWTProvider {
	return &JWTProvider{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (p *JWTProvider) CreateGuestToken(_ context.Context, sessionID, actorID, displayName string) (string, error) {
	now := time.Now()

	claims := &Claims{
		SessionID:   sessionID,
		ActorID:     actorID,
		ActorType:   "guest",
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			Subject:   actorID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(p.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secret)
}

func (p *JWTProvider) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return p.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
