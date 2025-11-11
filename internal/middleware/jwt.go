package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type JWTMiddleware struct {
	secret string
}

func NewJWTMiddleware(secret string) *JWTMiddleware {
	return &JWTMiddleware{secret: secret}
}

func (j *JWTMiddleware) Name() string {
	return "jwt"
}

// Process 校验 JWT Token
func (j *JWTMiddleware) Process(w http.ResponseWriter, r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")

	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Missing or invalid Authorization header", http.StatusUnauthorized)
		return false
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 校验签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		log.Printf("JWT validation failed: %v", err)
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		return false
	}

	if !token.Valid {
		http.Error(w, "Unauthorized: Token is invalid", http.StatusUnauthorized)
		return false
	}

	// 鉴权成功，可以将用户信息存入 context 供后续服务使用
	// r = r.WithContext(context.WithValue(r.Context(), "user", token.Claims))

	return true
}
