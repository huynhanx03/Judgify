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
		Compile:    []string{"sh", "-c", "GOPATH=/tmp/gopath GOCACHE=/tmp/go-build GONOSUMDB=* go mod init solution > /dev/null 2>&1 && GOPATH=/tmp/gopath GOCACHE=/tmp/go-build GONOSUMDB=* go build -o main ."},
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

	args := append([]string{"exec", "-w", "/sandbox", containerID}, cfg.Compile...)
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
// Optimized: uses a single docker exec call that resets memory peak, runs the program,
// and reads peak memory — reducing overhead from 4-6 docker exec calls to 1.
func Execute(ctx context.Context, containerID, language, input string, timeLimitMs, memoryLimitKb int) RunResult {
	cfg := languages[language]

	timeout := time.Duration(timeLimitMs+2000) * time.Millisecond
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build a single shell script that:
	// 1. Resets cgroup memory peak counter
	// 2. Runs the program
	// 3. Captures exit code
	// 4. Reads peak memory
	// 5. Outputs: EXIT_CODE\nMEMORY_BYTES\n then program stdout before that
	runCmd := strings.Join(cfg.RunCmd, " ")
	script := fmt.Sprintf(`
# Reset cgroup memory peak (best-effort, silent fail)
echo 0 > /sys/fs/cgroup/memory.peak 2>/dev/null
echo 0 > /sys/fs/cgroup/memory/memory.max_usage_in_bytes 2>/dev/null

# Run the program, capture stdout to temp file
%s < /dev/stdin > /tmp/_out 2> /tmp/_err
EXIT_CODE=$?

# Read peak memory (cgroups v2 first, then v1)
MEM=$(cat /sys/fs/cgroup/memory.peak 2>/dev/null || cat /sys/fs/cgroup/memory/memory.max_usage_in_bytes 2>/dev/null || echo 0)

# Output: program stdout, then markers with metadata
cat /tmp/_out
printf '\n===JUDGIFY_META===\n'
printf '%%d\n%%s\n' "$EXIT_CODE" "$MEM"
cat /tmp/_err >&2
`, runCmd)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(execCtx, "docker", "exec", "-i", "-w", "/sandbox", containerID, "sh", "-c", script)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	timeMs := int(elapsed.Milliseconds())

	// TLE: context deadline exceeded
	if execCtx.Err() == context.DeadlineExceeded {
		return RunResult{
			TimeMs:   timeLimitMs,
			MemoryKb: 0,
			Status:   "time_limit_exceeded",
		}
	}

	// Parse output: split by marker to get program output and metadata
	output, exitCode, memoryKb := parseExecutionOutput(stdout.String())

	if err != nil {
		// MLE: exit code 137 = OOM killed by Docker
		if exitCode == 137 || getExitCode(err) == 137 {
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
			Output:   output,
			TimeMs:   timeMs,
			MemoryKb: memoryKb,
			Status:   "memory_limit_exceeded",
		}
	}

	return RunResult{
		Output:   output,
		TimeMs:   timeMs,
		MemoryKb: memoryKb,
		Status:   "ok",
	}
}

// parseExecutionOutput splits the combined output into program output, exit code, and memory.
// Format: <program_output>\n===JUDGIFY_META===\n<exit_code>\n<memory_bytes>\n
func parseExecutionOutput(raw string) (output string, exitCode int, memoryKb int) {
	marker := "\n===JUDGIFY_META===\n"
	idx := strings.LastIndex(raw, marker)
	if idx == -1 {
		// No marker found — probably OOM killed before script completed
		return raw, -1, 0
	}

	output = raw[:idx]
	meta := raw[idx+len(marker):]
	lines := strings.SplitN(strings.TrimSpace(meta), "\n", 2)

	if len(lines) >= 1 {
		exitCode, _ = strconv.Atoi(strings.TrimSpace(lines[0]))
	}
	if len(lines) >= 2 {
		memBytes, _ := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
		memoryKb = int(memBytes / 1024)
	}

	return output, exitCode, memoryKb
}

// getExitCode extracts exit code from exec error. Returns -1 if not available.
func getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
