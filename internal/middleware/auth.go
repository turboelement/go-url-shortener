package middleware

import (
	"context"
	"net/http"

	"go-url-shortener/internal/auth"

	"go.uber.org/zap"
)

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
				auth.SetAuthCookie(w, userID, secret)
				logger.Info("Generated new user ID", zap.String("user_id", userID))
			} else if userID == "" {
				newUserID, err := auth.GenerateUserID()
				if err != nil {
					logger.Error("Failed to generate user ID", zap.Error(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				userID = newUserID
				auth.SetAuthCookie(w, userID, secret)
				logger.Info("Invalid cookie signature, generated new user ID", zap.String("user_id", userID))
			}

			ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	return userID, ok
}
