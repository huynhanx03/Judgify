package http

import (
	"bytes"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/tenantctx"
)

// RequestTransactionMiddleware gives tenant-bound API work one transaction
// and sets the transaction-local RLS key before any handler query runs.
func RequestTransactionMiddleware(database *internalent.EntClient, optional bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := tenantctx.FromContext(c.Request.Context())
		if !ok {
			if optional {
				c.Next()
				return
			}
			response.ErrorResponse(c, apperr.CodeInvalidID, apperr.New(apperr.CodeInvalidID, "a concrete tenant is required", nil))
			c.Abort()
			return
		}
		_, concrete := scope.TenantID()
		if !concrete && !(scope.IsAll() && AllowsAllScope(c.Request.Method, c.FullPath())) {
			response.ErrorResponse(c, apperr.CodeInvalidID, apperr.New(apperr.CodeInvalidID, "a concrete tenant is required", nil))
			c.Abort()
			return
		}

		originalWriter := c.Writer
		buffered := &tenantResponseWriter{ResponseWriter: originalWriter, status: http.StatusOK}
		c.Writer = buffered
		committed := false
		defer func() {
			c.Writer = originalWriter
			if recovered := recover(); recovered != nil {
				panic(recovered)
			}
			if committed {
				buffered.flushTo(originalWriter)
			}
		}()

		err := database.DoInTx(c.Request.Context(), func(txctx context.Context) error {
			_ = database.DB(txctx) // installs the transaction-local RLS scope
			c.Request = c.Request.WithContext(txctx)
			c.Next()
			return nil
		})
		if err != nil {
			c.Writer = originalWriter
			_ = c.Error(err)
			response.ErrorResponse(c, apperr.CodeInternalServer, apperr.New(apperr.CodeInternalServer, "tenant request failed", err))
			c.Abort()
			return
		}
		committed = true
	}
}

type tenantResponseWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *tenantResponseWriter) WriteHeader(code int)              { w.status = code }
func (w *tenantResponseWriter) WriteHeaderNow()                   {}
func (w *tenantResponseWriter) Write(p []byte) (int, error)       { return w.body.Write(p) }
func (w *tenantResponseWriter) WriteString(s string) (int, error) { return w.body.WriteString(s) }
func (w *tenantResponseWriter) Status() int                       { return w.status }
func (w *tenantResponseWriter) Size() int                         { return w.body.Len() }
func (w *tenantResponseWriter) Written() bool                     { return w.body.Len() > 0 || w.status != http.StatusOK }
func (w *tenantResponseWriter) Flush()                            {}

func (w *tenantResponseWriter) flushTo(destination gin.ResponseWriter) {
	for key, values := range w.Header() {
		destination.Header()[key] = append([]string(nil), values...)
	}
	destination.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = destination.Write(w.body.Bytes())
	}
}
