package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"
	"github.com/huynhanx03/judgify/pkg/utils"
)

// Authentication middleware validates the JWT token and sets UserID + Username in the context.
func Authentication(publicKey interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(constraints.HeaderAuthorization)
		if authHeader == "" {
			response.ErrorResponse(c, response.CodeUnauthorized, apperr.New(response.CodeUnauthorized, "missing authorization header", http.StatusUnauthorized, nil))
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != constraints.TokenTypeBearer {
			response.ErrorResponse(c, response.CodeUnauthorized, apperr.New(response.CodeUnauthorized, "invalid authorization header format", http.StatusUnauthorized, nil))
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse token
		token, err := jwt.ParseWithClaims(tokenString, &utils.Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return publicKey, nil
		})

		if err != nil || !token.Valid {
			response.ErrorResponse(c, response.CodeUnauthorized, apperr.New(response.CodeUnauthorized, "invalid or expired token", http.StatusUnauthorized, nil))
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*utils.Claims); ok {
			ctx := c.Request.Context()
			ctx = context.WithValue(ctx, constraints.ContextKeyClaims, claims)
			ctx = context.WithValue(ctx, constraints.ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, constraints.ContextKeyUsername, claims.Username)
			c.Request = c.Request.WithContext(ctx)
		} else {
			response.ErrorResponse(c, response.CodeUnauthorized, apperr.New(response.CodeUnauthorized, "invalid token claims", http.StatusUnauthorized, nil))
			c.Abort()
			return
		}

		c.Next()
	}
}
