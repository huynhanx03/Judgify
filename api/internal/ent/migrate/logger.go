package migration

import (
	"context"
	"errors"
	"fmt"
	"sync"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"go.uber.org/zap"

	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
)

// atlasLogger intentionally never records raw SQL. Migration files can later
// contain data backfills, so logging full statements would turn logs into a
// second data store. Version, file, statement position, CID, and the returned
// database error are enough to locate the exact checked-in statement.
type atlasLogger struct {
	ctx context.Context

	mu             sync.Mutex
	currentFile    string
	currentVersion string
	statement      int
}

func (logger *atlasLogger) Log(entry atlasmigrate.LogEntry) {
	if logger == nil {
		return
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()

	log := commonlogger.FromContext(logger.ctx).Named("migration")
	switch event := entry.(type) {
	case atlasmigrate.LogExecution:
		log.Info("migration execution started",
			zap.String("from_version", event.From),
			zap.String("to_version", event.To),
			zap.Int("file_count", len(event.Files)),
		)
	case atlasmigrate.LogFile:
		logger.currentFile = event.File.Name()
		logger.currentVersion = event.File.Version()
		logger.statement = event.Skip
		log.Info("migration file started",
			zap.String("migration_file", logger.currentFile),
			zap.String("migration_version", logger.currentVersion),
			zap.Int("statements_already_applied", event.Skip),
		)
	case atlasmigrate.LogStmt:
		logger.statement++
		fields := []zap.Field{
			zap.String("migration_file", logger.currentFile),
			zap.String("migration_version", logger.currentVersion),
			zap.Int("statement", logger.statement),
		}
		if event.Stmt != nil {
			fields = append(fields, zap.Int("statement_offset", event.Stmt.Pos))
		}
		log.Debug("migration statement executing", fields...)
	case atlasmigrate.LogDone:
		log.Info("migration execution completed")
	case atlasmigrate.LogError:
		log.Error("migration execution failed",
			zap.String("migration_file", logger.currentFile),
			zap.String("migration_version", logger.currentVersion),
			zap.Int("statement", logger.statement),
			zap.String("error_type", errorType(event.Error)),
			zap.Error(redactMigrationError(event.Error)),
		)
	case atlasmigrate.LogChecks:
		log.Debug("migration checks started",
			zap.String("check_group", event.Name),
			zap.Int("check_count", len(event.Stmts)),
		)
	case atlasmigrate.LogCheck:
		fields := []zap.Field{zap.Bool("passed", event.Error == nil)}
		if event.Error != nil {
			fields = append(fields,
				zap.String("error_type", errorType(event.Error)),
				zap.Error(redactMigrationError(event.Error)),
			)
		}
		log.Debug("migration check completed", fields...)
	case atlasmigrate.LogChecksDone:
		if event.Error != nil {
			log.Error("migration checks failed",
				zap.String("error_type", errorType(event.Error)),
				zap.Error(redactMigrationError(event.Error)),
			)
			return
		}
		log.Debug("migration checks completed")
	}
}

type redactedError struct {
	message string
	cause   error
}

func (err *redactedError) Error() string { return err.message }
func (err *redactedError) Unwrap() error { return err.cause }

func redactedCause(message string, cause error) error {
	if cause == nil {
		return nil
	}
	return &redactedError{message: message, cause: cause}
}

func redactMigrationError(err error) error {
	if err == nil {
		return nil
	}
	var statementError *atlasmigrate.StmtExecError
	if errors.As(err, &statementError) {
		position := 0
		if statementError.Stmt != nil {
			position = statementError.Stmt.Pos
		}
		return &redactedError{
			message: fmt.Sprintf(
				"migration version %q failed at statement offset %d",
				statementError.Version,
				position,
			),
			cause: err,
		}
	}
	return err
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}
