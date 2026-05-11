// Package auth handles user identification via JWT-signed cookies.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	cookieName   = "user_id"
	cookieMaxAge = time.Hour * 24 * 30 // 30 days
)

// GenerateUserID creates a new random UUID for identifying a user.
func GenerateUserID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate user id: %w", err)
	}
	return id.String(), nil
}

type customClaims struct {
	jwt.RegisteredClaims
}

// SignUserID creates a signed JWT token containing the user ID.
func SignUserID(userID, secret string) (string, error) {
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cookieMaxAge)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// VerifyUserID validates a JWT token and returns the user ID if valid.
func VerifyUserID(tokenString, secret string) (string, bool) {
	claims := &customClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return "", false
	}

	return claims.Subject, true
}

// SetAuthCookie sets a signed user ID cookie on the HTTP response.
func SetAuthCookie(w http.ResponseWriter, userID, secret string) error {
	signedValue, err := SignUserID(userID, secret)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    signedValue,
		MaxAge:   int(cookieMaxAge.Seconds()),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Set to true in production with HTTPS
	}

	http.SetCookie(w, cookie)
	return nil
}

// GetUserIDFromCookie extracts and validates the user ID from the request cookie.
func GetUserIDFromCookie(r *http.Request, secret string) (string, bool, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		if err == http.ErrNoCookie {
			return "", false, nil
		}
		return "", false, fmt.Errorf("error reading cookie: %w", err)
	}

	userID, valid := VerifyUserID(cookie.Value, secret)
	if !valid {
		return "", true, nil
	}

	return userID, true, nil
}

type contextKey string

const userIDKey contextKey = "user_id"

// SetUserIDInContext stores the user ID in the request context.
func SetUserIDInContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserIDFromContext retrieves the user ID from the request context.
func GetUserIDFromContext(ctx context.Context) (string, error) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return "", fmt.Errorf("user ID not found in context")
	}

	userID, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("user ID in context has invalid type: %T, expected string", val)
	}

	if userID == "" {
		return "", fmt.Errorf("user ID in context is empty")
	}

	return userID, nil
}
