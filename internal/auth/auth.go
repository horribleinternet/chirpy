package auth

import (
	"net/http"
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
	tokenstr, err := tokenptr.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return tokenstr, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) { return []byte(tokenSecret), nil })
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
	fulltoken := headers.Get("Authorization")
	string.splitN()
}
