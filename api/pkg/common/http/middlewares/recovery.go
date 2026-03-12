package middlewares

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

// RecoveryMiddleware captures panics and returns a 500 error
func RecoveryMiddleware(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			// Check if the panic is an error or other type
			var appErr error
			if e, ok := err.(error); ok {
				appErr = e
			} else {
				appErr = fmt.Errorf("%v", err)
			}

			// Log the stack trace
			log.Printf("Panic recovered: %v\nStack: %s\n", appErr, string(debug.Stack()))

			// Return standardized error response
			response.ErrorResponse(c, response.CodeInternalServer, apperr.NewError(
				"RecoveryMiddleware",
				response.CodeInternalServer,
				"Internal Server Error",
				http.StatusInternalServerError,
				appErr,
			))
			// Ensure we abort the context to stop propagation
			c.Abort()
		}
	}()
	c.Next()
}
