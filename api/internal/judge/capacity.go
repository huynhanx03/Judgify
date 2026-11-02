package judge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
)

var ErrCapacityProbeInvalid = errors.New("judge capacity probe: unavailable")

type diskAvailability func(string) (int64, error)

// HostCapacityProbe is the worker-local pressure signal used before a durable
// PostgreSQL claim. It contains no authority: a stale optimistic snapshot can
// only reduce throughput because the database still enforces global limits.
type HostCapacityProbe struct {
	scheduler     *FairScheduler
	diskPath      string
	diskAvailable diskAvailability
}

func NewHostCapacityProbe(
	scheduler *FairScheduler,
	diskPath string,
) (*HostCapacityProbe, error) {
	if scheduler == nil || !filepath.IsAbs(diskPath) ||
		filepath.Clean(diskPath) != diskPath ||
		strings.ContainsAny(diskPath, "\x00\r\n") {
		return nil, ErrCapacityProbeInvalid
	}
	return &HostCapacityProbe{
		scheduler:     scheduler,
		diskPath:      diskPath,
		diskAvailable: hostDiskAvailable,
	}, nil
}

func (probe *HostCapacityProbe) Snapshot(
	ctx context.Context,
) (submissionservice.CapacitySnapshot, error) {
	if probe == nil || probe.scheduler == nil || probe.diskAvailable == nil ||
		ctx == nil {
		return submissionservice.CapacitySnapshot{}, ErrCapacityProbeInvalid
	}
	if err := ctx.Err(); err != nil {
		return submissionservice.CapacitySnapshot{}, err
	}
	available, err := probe.diskAvailable(probe.diskPath)
	if err != nil || available < 0 {
		return submissionservice.CapacitySnapshot{}, errors.Join(
			ErrCapacityProbeInvalid,
			err,
		)
	}
	return submissionservice.CapacitySnapshot{
		FreeSandboxes:      probe.scheduler.FreeExecutionSlots(),
		DiskAvailableBytes: available,
	}, nil
}

func hostDiskAvailable(path string) (int64, error) {
	var statistics unix.Statfs_t
	if err := unix.Statfs(path, &statistics); err != nil {
		return 0, err
	}
	if statistics.Bsize <= 0 {
		return 0, ErrCapacityProbeInvalid
	}
	blockSize := uint64(statistics.Bsize)
	availableBlocks := uint64(statistics.Bavail)
	if availableBlocks > uint64(1<<63-1)/blockSize {
		return 0, ErrCapacityProbeInvalid
	}
	return int64(availableBlocks * blockSize), nil
}

var _ submissionservice.CapacityProbe = (*HostCapacityProbe)(nil)
