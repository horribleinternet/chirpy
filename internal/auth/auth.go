package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 0)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := jwt.NumericDate{Time: time.Now()}
	expire := jwt.NumericDate{Time: now.Time.Add(expiresIn)}
	claim := jwt.RegisteredClaims{Issuer: "chirpy", IssuedAt: &now, ExpiresAt: &expire, Subject: userID.String()}
	tokenptr := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	byteSecret, err := base64.StdEncoding.DecodeString(tokenSecret)
	if err != nil {
		return "", err
	}
	tokenstr, err := tokenptr.SignedString(byteSecret)
	if err != nil {
		return "", err
	}
	return tokenstr, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	byteSecret, err := base64.StdEncoding.DecodeString(tokenSecret)
	if err != nil {
		return uuid.UUID{}, err
	}
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) { return byteSecret, nil })
	if err != nil {
		return uuid.UUID{}, err
	}
	uuidstr, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, err
	}
	return uuid.Parse(uuidstr)
}

func GetBearerToken(headers http.Header) (string, error) {
	headerTok := strings.SplitN(headers.Get("Authorization"), " ", 2)
	if len(headerTok) < 2 || len(headerTok[1]) == 0 {
		return "", fmt.Errorf("valid bearer token not found in header")
	}
	return headerTok[1], nil
}

func MakeRefreshToken() (string, error) {
	rand.Read()
}
