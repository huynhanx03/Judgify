package middlewares

import "github.com/gin-gonic/gin"

// Compose chains multiple Gin middlewares into a single HandlerFunc.
// The middlewares execute in the order provided, wrapping the final handler.
//
// Usage with Gin router:
//
//	r.GET("/public", middlewares.Compose(
//	    middlewares.RateLimit(publicCfg),
//	)(publicHandler))
//
//	r.POST("/admin", middlewares.Compose(
//	    middlewares.Authentication(key),
//	    middlewares.RateLimit(adminCfg),
//	    middlewares.CircuitBreakerMiddleware(cb),
//	)(adminHandler))
func Compose(middlewares ...gin.HandlerFunc) func(gin.HandlerFunc) gin.HandlerFunc {
	return func(final gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			// Build a chain: each middleware calls c.Next() which
			// advances to the next handler in Gin's internal chain.
			handlers := make([]gin.HandlerFunc, 0, len(middlewares)+1)
			handlers = append(handlers, middlewares...)
			handlers = append(handlers, final)

			// Inject our chain into the Gin context and start execution.
			c.Set(composeChainKey, &composeChain{
				handlers: handlers,
				index:    0,
			})
			executeCompose(c)
		}
	}
}

// ComposeHandlers combines multiple handlers into one for use with Gin route registration.
// Unlike Compose, this does not wrap a final handler — it chains handlers directly.
//
// Usage:
//
//	r.GET("/path", middlewares.ComposeHandlers(
//	    middlewares.RateLimit(cfg),
//	    middlewares.Authentication(key),
//	    myHandler,
//	)...)
func ComposeHandlers(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	return handlers
}

const composeChainKey = "__compose_chain"

type composeChain struct {
	handlers []gin.HandlerFunc
	index    int
}

func executeCompose(c *gin.Context) {
	val, exists := c.Get(composeChainKey)
	if !exists {
		return
	}

	chain := val.(*composeChain)
	if chain.index >= len(chain.handlers) {
		return
	}

	current := chain.index
	chain.index++
	chain.handlers[current](c)
}
