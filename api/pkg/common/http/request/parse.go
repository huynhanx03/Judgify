package request

import (
	"io"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/common/http/validation"
)

// ParseRequest parses and validates the request body
func ParseRequest[T any](c *gin.Context) (*T, error) {
	var req T

	// Try to bind URI params (optional, ignore error if no tags)
	_ = c.ShouldBindUri(&req)

	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		return nil, apperr.New(response.CodeParamInvalid, err.Error(), err)
	}

	if ok, msg := validation.IsRequestValid(req); !ok {
		return nil, apperr.New(response.CodeValidationFailed, string(msg), nil)
	}

	return &req, nil
}
