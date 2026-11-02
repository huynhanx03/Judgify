package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	identityhttp "github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
)

var ErrInvalidProfileHandler = errors.New("profile http: invalid handler")

type Reader interface {
	Get(context.Context, uuid.UUID) (*identitydto.ProfileResponse, error)
}

type Handler struct {
	reader Reader
}

func NewHandler(reader Reader) (*Handler, error) {
	if reader == nil {
		return nil, ErrInvalidProfileHandler
	}
	return &Handler{reader: reader}, nil
}

func (handler *Handler) GetProfile(c *gin.Context) {
	if handler == nil || handler.reader == nil || c == nil || c.Request == nil {
		if c != nil {
			if c.Request == nil {
				c.AbortWithStatus(nethttp.StatusInternalServerError)
				return
			}
			fail(c, apperr.New(apperr.CodeInternalServer, "profile unavailable", ErrInvalidProfileHandler))
		}
		return
	}
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		fail(c, apperr.New(apperr.CodeUnauthorized, "authentication required", nil))
		return
	}
	profile, err := handler.reader.Get(c.Request.Context(), userID)
	if err != nil {
		fail(c, err)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, profile)
}

func fail(c *gin.Context, err error) {
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}

var _ identityhttp.ProfileHandler = (*Handler)(nil)
