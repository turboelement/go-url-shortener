package middleware

import (
	"net/http"

	"go-url-shortener/internal/auth"

	"go.uber.org/zap"
)

// AuthMiddleware sets or validates the user cookie on every request.
func AuthMiddleware(secret string, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, hasCookie, err := auth.GetUserIDFromCookie(r, secret)

			if err != nil {
				logger.Error("Error reading cookie", zap.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if !hasCookie {
				newUserID, err := auth.GenerateUserID()
				if err != nil {
					logger.Error("Failed to generate user ID", zap.Error(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				userID = newUserID
				if err := auth.SetAuthCookie(w, userID, secret); err != nil {
					logger.Error("Failed to set auth cookie", zap.Error(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				logger.Info("Generated new user ID", zap.String("user_id", userID))
			} else if userID == "" {
				newUserID, err := auth.GenerateUserID()
				if err != nil {
					logger.Error("Failed to generate user ID", zap.Error(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				userID = newUserID
				if err := auth.SetAuthCookie(w, userID, secret); err != nil {
					logger.Error("Failed to set auth cookie", zap.Error(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				logger.Info("Invalid cookie signature, generated new user ID", zap.String("user_id", userID))
			}

			ctx := auth.SetUserIDInContext(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext extracts the user ID from the request's context.
func GetUserIDFromContext(r *http.Request) (string, error) {
	return auth.GetUserIDFromContext(r.Context())
}
