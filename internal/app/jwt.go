package app

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret []byte
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{secret: []byte(secret)}
}

func (j *JWTManager) IssueToken(userID string, role Role) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"role":    string(role),
		"exp":     time.Now().UTC().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().UTC().Unix(),
	})
	return t.SignedString(j.secret)
}

func (j *JWTManager) ParseToken(token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil || !parsed.Valid {
		return Claims{}, ErrInvalidRequest
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, ErrInvalidRequest
	}

	userID, _ := claims["user_id"].(string)
	role, _ := claims["role"].(string)
	if userID == "" || (role != string(RoleAdmin) && role != string(RoleUser)) {
		return Claims{}, ErrInvalidRequest
	}
	return Claims{UserID: userID, Role: Role(role)}, nil
}
