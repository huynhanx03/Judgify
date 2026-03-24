package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RunResult holds the result of executing code against a single test case.
type RunResult struct {
	Output   string
	TimeMs   int
	MemoryKb int
	Status   string // "ok", "time_limit_exceeded", "memory_limit_exceeded", "runtime_error"
	Err      error
}

// CompileResult holds the result of compiling source code.
type CompileResult struct {
	Success      bool
	ErrorMessage string
}

// langConfig maps language to compile/run commands.
type langConfig struct {
	SourceFile string
	Compile    []string // empty = interpreted
	RunCmd     []string
}

var languages = map[string]langConfig{
	"cpp": {
		SourceFile: "main.cpp",
		Compile:    []string{"g++", "-O2", "-std=c++17", "-o", "main", "main.cpp"},
		RunCmd:     []string{"./main"},
	},
	"python": {
		SourceFile: "main.py",
		RunCmd:     []string{"python3", "main.py"},
	},
	"java": {
		SourceFile: "Main.java",
		Compile:    []string{"javac", "Main.java"},
		RunCmd:     []string{"java", "-cp", ".", "Main"},
	},
	"go": {
		SourceFile: "main.go",
		Compile:    []string{"go", "build", "-o", "main", "main.go"},
		RunCmd:     []string{"./main"},
	},
}

// CopySource writes source code into the container's /sandbox directory.
func CopySource(ctx context.Context, containerID, language, sourceCode string) error {
	cfg, ok := languages[language]
	if !ok {
		return fmt.Errorf("unsupported language: %s", language)
	}

	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", containerID, "sh", "-c",
		fmt.Sprintf("cat > /sandbox/%s", cfg.SourceFile))
	cmd.Stdin = strings.NewReader(sourceCode)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy source failed: %s: %w", string(out), err)
	}
	return nil
}

// Compile compiles the source code inside the container. Returns success=true for interpreted languages.
func Compile(ctx context.Context, containerID, language string, timeoutMs int) CompileResult {
	cfg := languages[language]
	if len(cfg.Compile) == 0 {
		return CompileResult{Success: true}
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]string{"exec", containerID}, cfg.Compile...)
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return CompileResult{
			Success:      false,
			ErrorMessage: stderr.String(),
		}
	}
	return CompileResult{Success: true}
}

// Execute runs the compiled program with the given input and time/memory limits.
func Execute(ctx context.Context, containerID, language, input string, timeLimitMs, memoryLimitKb int) RunResult {
	cfg := languages[language]

	// Reset cgroup memory peak counter before execution
	resetMemoryPeak(ctx, containerID)

	timeout := time.Duration(timeLimitMs+2000) * time.Millisecond // 2s buffer for overhead
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]string{"exec", "-i", containerID}, cfg.RunCmd...)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(execCtx, "docker", args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	timeMs := int(elapsed.Milliseconds())

	// Read peak memory from cgroup (zero overhead — just reads a file)
	memoryKb := readPeakMemoryKb(ctx, containerID)

	// TLE: context deadline exceeded
	if execCtx.Err() == context.DeadlineExceeded {
		return RunResult{
			TimeMs:   timeLimitMs,
			MemoryKb: memoryKb,
			Status:   "time_limit_exceeded",
		}
	}

	if err != nil {
		// MLE: exit code 137 = OOM killed by Docker
		if exitCode := getExitCode(err); exitCode == 137 {
			return RunResult{
				TimeMs:   timeMs,
				MemoryKb: memoryKb,
				Status:   "memory_limit_exceeded",
			}
		}

		return RunResult{
			TimeMs:   timeMs,
			MemoryKb: memoryKb,
			Status:   "runtime_error",
			Err:      fmt.Errorf("%s", stderr.String()),
		}
	}

	// MLE: program finished but exceeded memory limit
	if memoryLimitKb > 0 && memoryKb > memoryLimitKb {
		return RunResult{
			Output:   stdout.String(),
			TimeMs:   timeMs,
			MemoryKb: memoryKb,
			Status:   "memory_limit_exceeded",
		}
	}

	return RunResult{
		Output:   stdout.String(),
		TimeMs:   timeMs,
		MemoryKb: memoryKb,
		Status:   "ok",
	}
}

// getExitCode extracts exit code from exec error. Returns -1 if not available.
func getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// readPeakMemoryKb reads peak memory usage from cgroup inside the container.
// Tries cgroups v2 first, falls back to v1. Returns 0 if unavailable.
func readPeakMemoryKb(ctx context.Context, containerID string) int {
	// cgroups v2: memory.peak
	out, err := exec.CommandContext(ctx, "docker", "exec", containerID,
		"cat", "/sys/fs/cgroup/memory.peak").Output()
	if err == nil {
		if bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
			return int(bytes / 1024)
		}
	}

	// cgroups v1 fallback: memory.max_usage_in_bytes
	out, err = exec.CommandContext(ctx, "docker", "exec", containerID,
		"cat", "/sys/fs/cgroup/memory/memory.max_usage_in_bytes").Output()
	if err == nil {
		if bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
			return int(bytes / 1024)
		}
	}

	return 0
}

// resetMemoryPeak resets the cgroup memory peak counter before each test case.
func resetMemoryPeak(ctx context.Context, containerID string) {
	// cgroups v2: reset by writing 0 to memory.peak (Linux 6.7+, best-effort)
	_ = exec.CommandContext(ctx, "docker", "exec", containerID,
		"sh", "-c", "echo 0 > /sys/fs/cgroup/memory.peak 2>/dev/null").Run()

	// cgroups v1: reset by writing 0 to memory.max_usage_in_bytes
	_ = exec.CommandContext(ctx, "docker", "exec", containerID,
		"sh", "-c", "echo 0 > /sys/fs/cgroup/memory/memory.max_usage_in_bytes 2>/dev/null").Run()
}
