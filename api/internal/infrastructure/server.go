package infrastructure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/di"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server wraps the HTTP server.
type Server struct {
	engine *gin.Engine
}

// NewServer creates a new Server instance.
func NewServer(engine *gin.Engine) *Server {
	return &Server{
		engine: engine,
	}
}

// NewHTTPServer creates the HTTP server using global dependencies.
func NewHTTPServer() *Server {
	c := di.GlobalContainer

	// Create PermissionChecker with DB repos + local cache
	permChecker := middlewares.NewPermissionChecker(
		c.Identity.UserRepo,
		c.Identity.RoleRepo,
		c.Identity.PermissionRepo,
		global.Ember,
	)

	// Create router group with modular handlers from domain containers
	routerGroup := NewRouterGroup(
		c.Identity.IdentityHandler,
		c.Problem.ProblemHandlerGroup,
		c.Cultivation.CultivationHandlerGroup,
		c.Submission.SubmissionHandlerGroup,
		permChecker,
	)

	// Create Gin engine
	engine := NewEngine(routerGroup)

	return NewServer(engine)
}

// Run starts the HTTP server with graceful shutdown.
func (s *Server) Run() error {
	host := os.Getenv("SERVER_HOST")
	if host == "" {
		host = global.Config.Server.Host
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(global.Config.Server.Port)
	}

	address := fmt.Sprintf("%s:%s", host, port)

	srv := &http.Server{
		Addr:    address,
		Handler: s.engine,
	}

	go func() {
		global.LoggerZap.Info("Server starting",
			zap.String("address", address),
			zap.String("mode", global.Config.Server.Mode),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			global.LoggerZap.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	global.LoggerZap.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		global.LoggerZap.Error("Server forced to shutdown", zap.Error(err))
		return err
	}

	global.LoggerZap.Info("Server exited")

	// Flush any buffered log entries before exit
	_ = global.LoggerZap.Sync()
	return nil
}
