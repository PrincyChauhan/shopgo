package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"shopgo/models"
)

const tokenLifetime = 24 * time.Hour

// Claims is the data stored INSIDE the token. It is signed, not encrypted,
// so never put secrets in here. We keep just the user id and role.
type Claims struct {
	UserID uint        `json:"uid"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, user *models.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenLifetime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (interface{}, error) { return []byte(secret), nil },
		// Only accept the algorithm we actually use. This blocks "alg" tricks.
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}
