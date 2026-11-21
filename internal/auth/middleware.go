package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/sikozonpc/cinema/internal/utils"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"

func RequireAuth(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := r.Header.Get("Authorization")
			if !strings.HasPrefix(authorization, "Bearer ") {
				utils.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			claims, err := service.ValidateAccess(strings.TrimPrefix(authorization, "Bearer "))
			if err != nil {
				utils.WriteError(w, http.StatusUnauthorized, "invalid or expired access token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(service *Service, roles ...string) func(http.Handler) http.Handler {
	required := make(map[string]bool, len(roles))
	for _, role := range roles {
		required[role] = true
	}
	return func(next http.Handler) http.Handler {
		authenticated := RequireAuth(service)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, _ := r.Context().Value(claimsContextKey).(*Claims)
			if claims == nil || !required[claims.Role] {
				utils.WriteError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		}))
		return authenticated
	}
}

func ClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsContextKey).(*Claims)
	return claims
}
