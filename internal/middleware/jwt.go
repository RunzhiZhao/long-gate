package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func JWT(secret string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			authHeader := ctx.Request.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				ctx.Response.WriteHeader(http.StatusUnauthorized)
				ctx.Response.Write([]byte("Unauthorized"))
				ctx.Abort()
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})
			if err != nil {
				ctx.Response.WriteHeader(http.StatusUnauthorized)
				ctx.Response.Write([]byte("Unauthorized"))
				ctx.Abort()
				return
			}
			if !token.Valid {
				ctx.Response.WriteHeader(http.StatusUnauthorized)
				ctx.Response.Write([]byte("Unauthorized"))
				ctx.Abort()
				return
			}
			ctx.Set("jwt_claims", token.Claims)
			next(ctx)
		}
	}
}
