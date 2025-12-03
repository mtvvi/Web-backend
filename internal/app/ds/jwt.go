package ds

import (
	"backend/internal/app/role"

	"github.com/golang-jwt/jwt"
)

type JWTClaims struct {
	jwt.StandardClaims
	UserID uint      `json:"user_id"`
	Role   role.Role `json:"role"`
}
