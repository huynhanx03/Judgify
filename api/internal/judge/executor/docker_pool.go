package executor

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// DockerPool manages a pool of pre-created Docker containers for sandboxed code execution.
type DockerPool struct {
	mu         sync.Mutex
	containers chan string // buffered channel of available container IDs
	image      string
	network    bool // true = network enabled
	poolSize   int
	memoryMb   int // per-container memory limit in MB
	tmpfsSizeMb int // /sandbox tmpfs size in MB
}

// NewDockerPool creates and starts N containers from the given image.
// Containers are created in parallel for faster startup.
func NewDockerPool(ctx context.Context, image string, poolSize int, networkDisabled bool, memoryMb, tmpfsSizeMb int) (*DockerPool, error) {
	// Apply defaults
	if memoryMb <= 0 {
		memoryMb = 512
	}
	if tmpfsSizeMb <= 0 {
		tmpfsSizeMb = 64
	}

	pool := &DockerPool{
		containers:  make(chan string, poolSize),
		image:       image,
		network:     !networkDisabled,
		poolSize:    poolSize,
		memoryMb:    memoryMb,
		tmpfsSizeMb: tmpfsSizeMb,
	}

	// Create containers in parallel
	type result struct {
		id  string
		err error
	}
	results := make(chan result, poolSize)
	for i := 0; i < poolSize; i++ {
		go func(idx int) {
			id, err := pool.createContainer(ctx, idx)
			results <- result{id, err}
		}(i)
	}

	// Collect results
	var created []string
	for i := 0; i < poolSize; i++ {
		r := <-results
		if r.err != nil {
			// Cleanup already-created containers
			for _, id := range created {
				_ = stopAndRemove(ctx, id)
			}
			return nil, fmt.Errorf("failed to create container: %w", r.err)
		}
		created = append(created, r.id)
	}

	for _, id := range created {
		pool.containers <- id
	}

	return pool, nil
}

// Acquire gets a free container from the pool. Blocks if none available.
func (p *DockerPool) Acquire(ctx context.Context) (string, error) {
	select {
	case id := <-p.containers:
		// Health check: verify container is still running
		if !isContainerRunning(ctx, id) {
			// Replace dead container
			newID, err := p.recreateContainer(ctx, id)
			if err != nil {
				return "", fmt.Errorf("container %s dead, recreate failed: %w", id, err)
			}
			return newID, nil
		}
		return id, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Release cleans the container sandbox and returns it to the pool.
func (p *DockerPool) Release(ctx context.Context, containerID string) {
	// Clean up files left by previous execution using sh -c for glob expansion
	_ = dockerExec(ctx, containerID, "sh", "-c", "rm -rf /sandbox/* /tmp/*")
	p.containers <- containerID
}

// Shutdown stops and removes all containers in the pool.
func (p *DockerPool) Shutdown(ctx context.Context) {
	close(p.containers)
	for id := range p.containers {
		_ = stopAndRemove(ctx, id)
	}
}

func (p *DockerPool) createContainer(ctx context.Context, index int) (string, error) {
	name := fmt.Sprintf("judgify-sandbox-%d", index)

	// Remove existing container with same name (if leftover from previous run)
	_ = stopAndRemove(ctx, name)

	memFlag := fmt.Sprintf("--memory=%dm", p.memoryMb)
	sandboxTmpfs := fmt.Sprintf("/sandbox:rw,exec,size=%dm", p.tmpfsSizeMb)

	args := []string{
		"create",
		"--name", name,
		memFlag,
		"--cpus=1",
		"--pids-limit=128",
		"--read-only",
		"--tmpfs", sandboxTmpfs,
		"--tmpfs", "/tmp:rw,size=256m",
		"-w", "/sandbox",
	}

	if !p.network {
		args = append(args, "--network=none")
	}

	args = append(args, p.image, "sleep", "infinity")

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", string(out), err)
	}

	// Start the container
	if err := exec.CommandContext(ctx, "docker", "start", name).Run(); err != nil {
		return "", fmt.Errorf("failed to start container %s: %w", name, err)
	}

	return name, nil
}

// recreateContainer removes a dead container and creates a fresh one with the same name.
func (p *DockerPool) recreateContainer(ctx context.Context, name string) (string, error) {
	_ = stopAndRemove(ctx, name)

	// Extract index from name "judgify-sandbox-N"
	var index int
	fmt.Sscanf(name, "judgify-sandbox-%d", &index)

	return p.createContainer(ctx, index)
}

// isContainerRunning checks if a container is alive via a fast inspect.
func isContainerRunning(ctx context.Context, containerID string) bool {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", containerID).Output()
	if err != nil {
		return false
	}
	return len(out) >= 4 && out[0] == 't' // "true\n"
}

func stopAndRemove(ctx context.Context, name string) error {
	_ = exec.CommandContext(ctx, "docker", "stop", "-t", "2", name).Run()
	return exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

func dockerExec(ctx context.Context, containerID string, cmd ...string) error {
	args := append([]string{"exec", containerID}, cmd...)
	return exec.CommandContext(ctx, "docker", args...).Run()
}
