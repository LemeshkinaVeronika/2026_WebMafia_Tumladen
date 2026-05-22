package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/webmafia/tumladan/internal/model"
	jwtprovider "github.com/webmafia/tumladan/pkg/jwt"
)

type contextKey string

const (
	actorKey       contextKey = "actor"
	authSessionKey contextKey = "authSession"
)

type Auth struct {
	jwt *jwtprovider.JWTProvider
}

func NewAuth(jwt *jwtprovider.JWTProvider) *Auth {
	return &Auth{jwt: jwt}
}

func (a *Auth) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr, ok := BearerTokenFromRequest(r)
		if !ok {
			if r.Header.Get("Authorization") == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
			} else {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			}
			return
		}

		claims, err := a.jwt.ParseToken(tokenStr)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		actorID := claims.ActorID
		if actorID == "" {
			actorID = claims.RegisteredClaims.Subject
		}
		if actorID == "" {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		actor := model.Actor{
			ID:          actorID,
			Type:        model.ActorType(claims.ActorType),
			DisplayName: claims.DisplayName,
		}

		authSession := model.AuthSession{
			SessionID: claims.SessionID,
			Actor:     actor,
		}

		ctx := context.WithValue(r.Context(), actorKey, actor)
		ctx = context.WithValue(ctx, authSessionKey, authSession)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func BearerTokenFromRequest(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}

	return parts[1], true
}

func ActorFromContext(ctx context.Context) (model.Actor, bool) {
	actor, ok := ctx.Value(actorKey).(model.Actor)
	return actor, ok
}

func AuthSessionFromContext(ctx context.Context) (model.AuthSession, bool) {
	session, ok := ctx.Value(authSessionKey).(model.AuthSession)
	return session, ok
}
