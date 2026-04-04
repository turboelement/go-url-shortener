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
	CookieName   = "user_id"
	CookieMaxAge = time.Hour * 24 * 30 // 30 days
)

func GenerateUserID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate user id: %w", err)
	}
	return id.String(), nil
}

type CustomClaims struct {
	jwt.RegisteredClaims
}

func SignUserID(userID, secret string) (string, error) {
	claims := CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(CookieMaxAge)),
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

func VerifyUserID(tokenString, secret string) (string, bool) {
	claims := &CustomClaims{}

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

func SetAuthCookie(w http.ResponseWriter, userID, secret string) error {
	signedValue, err := SignUserID(userID, secret)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    signedValue,
		MaxAge:   int(CookieMaxAge.Seconds()),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Set to true in production with HTTPS
	}

	http.SetCookie(w, cookie)
	return nil
}

func GetUserIDFromCookie(r *http.Request, secret string) (string, bool, error) {
	cookie, err := r.Cookie(CookieName)
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

func SetUserIDInContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

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
