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
}

// NewDockerPool creates and starts N containers from the given image.
func NewDockerPool(ctx context.Context, image string, poolSize int, networkDisabled bool) (*DockerPool, error) {
	pool := &DockerPool{
		containers: make(chan string, poolSize),
		image:      image,
		network:    !networkDisabled,
		poolSize:   poolSize,
	}

	for i := 0; i < poolSize; i++ {
		id, err := pool.createContainer(ctx, i)
		if err != nil {
			pool.Shutdown(ctx)
			return nil, fmt.Errorf("failed to create container %d: %w", i, err)
		}
		pool.containers <- id
	}

	return pool, nil
}

// Acquire gets a free container from the pool. Blocks if none available.
func (p *DockerPool) Acquire(ctx context.Context) (string, error) {
	select {
	case id := <-p.containers:
		return id, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Release resets the container filesystem and returns it to the pool.
func (p *DockerPool) Release(ctx context.Context, containerID string) {
	// Clean up any files left by previous execution
	_ = dockerExec(ctx, containerID, "rm", "-rf", "/sandbox/*")
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

	args := []string{
		"create",
		"--name", name,
		"--memory=256m",
		"--cpus=1",
		"--pids-limit=64",
		"--read-only",
		"--tmpfs", "/sandbox:rw,exec,size=64m",
		"--tmpfs", "/tmp:rw,size=32m",
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

func stopAndRemove(ctx context.Context, name string) error {
	_ = exec.CommandContext(ctx, "docker", "stop", "-t", "2", name).Run()
	return exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

func dockerExec(ctx context.Context, containerID string, cmd ...string) error {
	args := append([]string{"exec", containerID}, cmd...)
	return exec.CommandContext(ctx, "docker", args...).Run()
}
