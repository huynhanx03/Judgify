package judge

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/pkg/common/workerpool"
	"github.com/huynhanx03/judgify/pkg/mq/forge"
	"github.com/huynhanx03/judgify/pkg/settings"

	"github.com/huynhanx03/judgify/internal/judge/comparator"
	"github.com/huynhanx03/judgify/internal/judge/executor"
	problemPorts "github.com/huynhanx03/judgify/internal/problem/ports"
	submissionPorts "github.com/huynhanx03/judgify/internal/submission/ports"
)

// JudgeJob is the MQ message payload.
type JudgeJob struct {
	SubmissionID int `json:"submission_id"`
}

// Worker consumes judge jobs from Forge MQ and processes them.
type Worker struct {
	consumer       *forge.Consumer
	pool           *workerpool.GenericPool[int]
	dockerPool     *executor.DockerPool
	submissionRepo submissionPorts.SubmissionRepository
	problemRepo    problemPorts.ProblemRepository
	testCaseRepo   problemPorts.TestCaseRepository
	config         settings.Judge
	logger         *zap.Logger
	stopCh         chan struct{}
}

// NewWorker creates a judge worker that consumes from MQ and dispatches to GenericPool.
func NewWorker(
	consumer *forge.Consumer,
	dockerPool *executor.DockerPool,
	submissionRepo submissionPorts.SubmissionRepository,
	problemRepo problemPorts.ProblemRepository,
	testCaseRepo problemPorts.TestCaseRepository,
	config settings.Judge,
) (*Worker, error) {
	w := &Worker{
		consumer:       consumer,
		dockerPool:     dockerPool,
		submissionRepo: submissionRepo,
		problemRepo:    problemRepo,
		testCaseRepo:   testCaseRepo,
		config:         config,
		logger:         global.LoggerZap.Named("judge"),
		stopCh:         make(chan struct{}),
	}

	pool, err := workerpool.NewGenericPool[int](config.WorkerCount, w.processSubmission)
	if err != nil {
		return nil, err
	}
	w.pool = pool

	return w, nil
}

// Start begins polling the MQ for judge jobs.
func (w *Worker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			default:
				records, err := w.consumer.Poll(10)
				if err != nil {
					w.logger.Error("poll error", zap.Error(err))
					time.Sleep(100 * time.Millisecond)
					continue
				}

				if len(records) == 0 {
					time.Sleep(50 * time.Millisecond)
					continue
				}

				for _, rec := range records {
					var job JudgeJob
					if err := json.Unmarshal(rec.Value, &job); err != nil {
						w.logger.Error("invalid job payload", zap.Error(err))
						continue
					}
					if err := w.pool.Invoke(job.SubmissionID); err != nil {
						w.logger.Error("pool invoke error", zap.Int("submission_id", job.SubmissionID), zap.Error(err))
					}
				}

				if err := w.consumer.Commit(); err != nil {
					w.logger.Error("commit error", zap.Error(err))
				}
			}
		}
	}()
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	close(w.stopCh)
	w.pool.Release()
}

// processSubmission is the GenericPool handler — judges one submission.
func (w *Worker) processSubmission(submissionID int) {
	ctx := context.Background()
	log := w.logger.With(zap.Int("submission_id", submissionID))

	// Load submission
	sub, err := w.submissionRepo.Get(ctx, submissionID)
	if err != nil {
		log.Error("submission not found", zap.Error(err))
		return
	}

	log = log.With(zap.Int("problem_id", sub.ProblemID), zap.String("language", sub.Language))

	// Mark as judging
	sub.Status = "judging"
	_ = w.submissionRepo.Update(ctx, sub)

	// Load problem for limits
	problem, err := w.problemRepo.Get(ctx, sub.ProblemID)
	if err != nil {
		log.Error("problem not found", zap.Error(err))
		sub.Status = "runtime_error"
		_ = w.submissionRepo.Update(ctx, sub)
		return
	}

	// Load test cases
	testCases, err := w.testCaseRepo.FindByProblemID(ctx, sub.ProblemID)
	if err != nil {
		log.Error("failed to load test cases", zap.Error(err))
		sub.Status = "runtime_error"
		_ = w.submissionRepo.Update(ctx, sub)
		return
	}

	sub.TotalCount = len(testCases)

	// Acquire a container from pool
	containerID, err := w.dockerPool.Acquire(ctx)
	if err != nil {
		log.Error("acquire container error", zap.Error(err))
		sub.Status = "runtime_error"
		_ = w.submissionRepo.Update(ctx, sub)
		return
	}
	defer w.dockerPool.Release(ctx, containerID)

	// Copy source code
	if err := executor.CopySource(ctx, containerID, sub.Language, sub.SourceCode); err != nil {
		log.Error("copy source error", zap.Error(err))
		sub.Status = "runtime_error"
		_ = w.submissionRepo.Update(ctx, sub)
		return
	}

	// Compile
	compileResult := executor.Compile(ctx, containerID, sub.Language, w.config.CompileTimeoutMs)
	if !compileResult.Success {
		sub.Status = "compile_error"
		sub.ErrorMessage = &compileResult.ErrorMessage
		_ = w.submissionRepo.Update(ctx, sub)
		log.Info("compile error", zap.String("status", "compile_error"))
		return
	}

	// Run each test case
	var maxTimeMs, maxMemoryKb int
	for _, tc := range testCases {
		result := executor.Execute(ctx, containerID, sub.Language, tc.Input, problem.TimeLimitMs, problem.MemoryLimitKb)

		if result.TimeMs > maxTimeMs {
			maxTimeMs = result.TimeMs
		}
		if result.MemoryKb > maxMemoryKb {
			maxMemoryKb = result.MemoryKb
		}

		switch result.Status {
		case "time_limit_exceeded":
			sub.Status = "time_limit_exceeded"
			sub.TimeMs = &maxTimeMs
			sub.MemoryKb = &maxMemoryKb
			_ = w.submissionRepo.Update(ctx, sub)
			log.Info("judged", zap.String("status", sub.Status), zap.Int("time_ms", maxTimeMs))
			return
		case "memory_limit_exceeded":
			sub.Status = "memory_limit_exceeded"
			sub.TimeMs = &maxTimeMs
			sub.MemoryKb = &maxMemoryKb
			_ = w.submissionRepo.Update(ctx, sub)
			log.Info("judged", zap.String("status", sub.Status), zap.Int("memory_kb", maxMemoryKb))
			return
		case "runtime_error":
			sub.Status = "runtime_error"
			if result.Err != nil {
				errMsg := result.Err.Error()
				sub.ErrorMessage = &errMsg
			}
			sub.TimeMs = &maxTimeMs
			sub.MemoryKb = &maxMemoryKb
			_ = w.submissionRepo.Update(ctx, sub)
			log.Info("judged", zap.String("status", sub.Status))
			return
		case "ok":
			if comparator.CompareOutputCF(result.Output, tc.ExpectedOutput) {
				sub.PassedCount++
			} else {
				sub.Status = "wrong_answer"
				sub.TimeMs = &maxTimeMs
				sub.MemoryKb = &maxMemoryKb
				_ = w.submissionRepo.Update(ctx, sub)
				log.Info("judged", zap.String("status", sub.Status), zap.Int("passed", sub.PassedCount), zap.Int("total", sub.TotalCount))
				return
			}
		}
	}

	// All test cases passed
	sub.Status = "accepted"
	sub.TimeMs = &maxTimeMs
	sub.MemoryKb = &maxMemoryKb
	_ = w.submissionRepo.Update(ctx, sub)
	log.Info("judged", zap.String("status", "accepted"), zap.Int("time_ms", maxTimeMs), zap.Int("memory_kb", maxMemoryKb))
}
