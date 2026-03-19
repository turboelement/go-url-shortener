package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func SignUserID(userID, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(userID))
	signature := hex.EncodeToString(h.Sum(nil))
	return userID + "." + signature
}

func VerifyUserID(cookieValue, secret string) (string, bool) {
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 {
		return "", false
	}

	userID := parts[0]
	providedSignature := parts[1]

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(userID))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return "", false
	}

	return userID, true
}

func SetAuthCookie(w http.ResponseWriter, userID, secret string) {
	signedValue := SignUserID(userID, secret)

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

type ContextKey string

const UserIDKey ContextKey = "user_id"
