package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hashed_password, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}

	return hashed_password, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return match, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	mySigningKey := []byte(tokenSecret)
	stringID := userID.String()
	now := time.Now().UTC()
	expirationTime := now.Add(expiresIn)
	expiresAt := jwt.NewNumericDate(expirationTime)

	claims := &jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: expiresAt,
		Subject:   stringID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(mySigningKey)
	return ss, err

}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	if !token.Valid {
		return uuid.Nil, errors.New("token not valid")

	}
	registeredClaims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return uuid.Nil, errors.New("unknown claims")
	}
	userID, err := uuid.Parse(registeredClaims.Subject)
	return userID, err
}

func GetBearerToken(headers http.Header) (string, error) {
	auth := headers.Get("Authorization")
	if auth == "" {
		return "", errors.New("missing header")
	}
	if strings.HasPrefix(auth, "Bearer ") {
		tokenString := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if tokenString == "" {
			return "", errors.New("missing token")
		}

		return tokenString, nil

	} else {
		return "", errors.New("wrong header")
	}

}

func MakeRefreshToken() string {
	token := make([]byte, 32)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)
	return tokenStr
}
