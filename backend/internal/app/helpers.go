package app

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func secureRandomString(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func generateJWT(ID uuid.UUID) (string, string, error) {
	now := time.Now()

	// Новый Access AccessToken
	accessClaims := jwt.MapClaims{
		"sub": ID.String(),
		"exp": now.Add(15 * time.Minute).Unix(),
		"iat": now.Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	// Новый Refresh AccessToken
	newRefreshTokenString := secureRandomString(32)

	return accessTokenString, newRefreshTokenString, nil
}
