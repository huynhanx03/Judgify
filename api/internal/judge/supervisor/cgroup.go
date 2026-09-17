package supervisor

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maximumCgroupFileBytes = 16 << 10

var ErrUntrustedAccounting = errors.New("sandbox supervisor: untrusted cgroup accounting")

type ResourceSnapshot struct {
	CPUUsage           time.Duration
	MemoryCurrentBytes int64
	MemoryPeakBytes    int64
	OOMCount           uint64
	OOMKillCount       uint64
	CPUThrottledCount  uint64
	PIDLimitCount      uint64
}

type ResourceDelta struct {
	CPUUsage           time.Duration
	MemoryCurrentBytes int64
	MemoryPeakBytes    int64
	OOMEvents          uint64
	OOMKills           uint64
	CPUThrottledEvents uint64
	PIDLimitEvents     uint64
}

type CgroupAccount interface {
	ID() string
	Snapshot(ctx context.Context) (ResourceSnapshot, error)
	Close() error
}

type CgroupResolver interface {
	Resolve(ctx context.Context, containerID string, pid int) (CgroupAccount, error)
}

type LinuxCgroupV2Resolver struct {
	procRoot   string
	cgroupRoot string
}

func NewLinuxCgroupV2Resolver(procRoot string, cgroupRoot string) (*LinuxCgroupV2Resolver, error) {
	if !filepath.IsAbs(procRoot) || !filepath.IsAbs(cgroupRoot) ||
		filepath.Clean(procRoot) != procRoot || filepath.Clean(cgroupRoot) != cgroupRoot {
		return nil, ErrUntrustedAccounting
	}
	return &LinuxCgroupV2Resolver{procRoot: procRoot, cgroupRoot: cgroupRoot}, nil
}

func NewProductionCgroupV2Resolver() *LinuxCgroupV2Resolver {
	resolver, _ := NewLinuxCgroupV2Resolver("/proc", "/sys/fs/cgroup")
	return resolver
}

func (resolver *LinuxCgroupV2Resolver) Resolve(
	ctx context.Context,
	containerID string,
	pid int,
) (CgroupAccount, error) {
	if resolver == nil || ctx == nil || ctx.Err() != nil ||
		containerID == "" || pid <= 0 {
		return nil, ErrUntrustedAccounting
	}
	membership, err := readHostFileBounded(
		filepath.Join(resolver.procRoot, strconv.Itoa(pid), "cgroup"),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: read process membership: %v", ErrUntrustedAccounting, err)
	}
	relative, err := parseUnifiedMembership(membership)
	if err != nil {
		return nil, err
	}
	if !cgroupPathOwnsContainer(relative, containerID) {
		return nil, ErrUntrustedAccounting
	}
	root, err := os.OpenRoot(resolver.cgroupRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: open cgroup root: %v", ErrUntrustedAccounting, err)
	}
	account := &linuxCgroupAccount{
		root:     root,
		relative: relative,
		opaqueID: opaqueCgroupID(containerID, relative),
	}
	if _, err := account.Snapshot(ctx); err != nil {
		_ = root.Close()
		return nil, err
	}
	return account, nil
}

type linuxCgroupAccount struct {
	root     *os.Root
	relative string
	opaqueID string
}

func (account *linuxCgroupAccount) ID() string {
	if account == nil {
		return ""
	}
	return account.opaqueID
}

func (account *linuxCgroupAccount) Snapshot(ctx context.Context) (ResourceSnapshot, error) {
	if account == nil || account.root == nil || ctx == nil {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	if err := ctx.Err(); err != nil {
		return ResourceSnapshot{}, err
	}
	cpu, err := account.readKeyValues("cpu.stat")
	if err != nil {
		return ResourceSnapshot{}, err
	}
	usage, ok := cpu["usage_usec"]
	if !ok || usage > uint64((time.Duration(1<<63-1))/time.Microsecond) {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	cpuThrottled, ok := cpu["nr_throttled"]
	if !ok {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	current, err := account.readScalar("memory.current")
	if err != nil {
		return ResourceSnapshot{}, err
	}
	peak, err := account.readScalar("memory.peak")
	if err != nil || peak < current {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	memoryEvents, err := account.readKeyValues("memory.events")
	if err != nil {
		return ResourceSnapshot{}, err
	}
	oomKills, ok := memoryEvents["oom_kill"]
	if !ok {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	oom, ok := memoryEvents["oom"]
	if !ok {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	pidEvents, err := account.readKeyValues("pids.events")
	if err != nil {
		return ResourceSnapshot{}, err
	}
	pidLimit, ok := pidEvents["max"]
	if !ok {
		return ResourceSnapshot{}, ErrUntrustedAccounting
	}
	return ResourceSnapshot{
		CPUUsage:           time.Duration(usage) * time.Microsecond,
		MemoryCurrentBytes: int64(current),
		MemoryPeakBytes:    int64(peak),
		OOMCount:           oom,
		OOMKillCount:       oomKills,
		CPUThrottledCount:  cpuThrottled,
		PIDLimitCount:      pidLimit,
	}, nil
}

func (account *linuxCgroupAccount) Close() error {
	if account == nil || account.root == nil {
		return nil
	}
	err := account.root.Close()
	account.root = nil
	return err
}

func (account *linuxCgroupAccount) readScalar(name string) (uint64, error) {
	value, err := account.read(name)
	if err != nil {
		return 0, err
	}
	text := strings.TrimSuffix(string(value), "\n")
	if text == "" || strings.ContainsAny(text, " \t\r\n") || text == "max" {
		return 0, ErrUntrustedAccounting
	}
	number, err := strconv.ParseUint(text, 10, 63)
	if err != nil {
		return 0, ErrUntrustedAccounting
	}
	return number, nil
}

func (account *linuxCgroupAccount) readKeyValues(name string) (map[string]uint64, error) {
	value, err := account.read(name)
	if err != nil {
		return nil, err
	}
	result := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(string(value)))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 2 || parts[0] == "" {
			return nil, ErrUntrustedAccounting
		}
		if _, duplicate := result[parts[0]]; duplicate {
			return nil, ErrUntrustedAccounting
		}
		number, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, ErrUntrustedAccounting
		}
		result[parts[0]] = number
	}
	if err := scanner.Err(); err != nil || len(result) == 0 {
		return nil, ErrUntrustedAccounting
	}
	return result, nil
}

func (account *linuxCgroupAccount) read(name string) ([]byte, error) {
	filename := filepath.ToSlash(filepath.Join(account.relative, name))
	file, err := account.root.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("%w: open %s: %v", ErrUntrustedAccounting, name, err)
	}
	defer func() { _ = file.Close() }()
	value, err := io.ReadAll(io.LimitReader(file, maximumCgroupFileBytes+1))
	if err != nil || len(value) == 0 || len(value) > maximumCgroupFileBytes {
		return nil, ErrUntrustedAccounting
	}
	return value, nil
}

func parseUnifiedMembership(value []byte) (string, error) {
	text := string(value)
	if !strings.HasSuffix(text, "\n") {
		return "", ErrUntrustedAccounting
	}
	text = strings.TrimSuffix(text, "\n")
	if text == "" || strings.ContainsAny(text, "\r\n\x00") ||
		!strings.HasPrefix(text, "0::/") {
		return "", ErrUntrustedAccounting
	}
	relative := strings.TrimPrefix(text, "0::/")
	if relative == "" {
		relative = "."
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(clean) || filepath.ToSlash(clean) != relative {
		return "", ErrUntrustedAccounting
	}
	return clean, nil
}

func cgroupPathOwnsContainer(relative string, containerID string) bool {
	if !containerIDPattern.MatchString(containerID) {
		return false
	}
	for _, component := range strings.Split(filepath.ToSlash(relative), "/") {
		if strings.Contains(component, containerID) {
			return true
		}
	}
	return false
}

func readHostFileBounded(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	value, err := io.ReadAll(io.LimitReader(file, maximumCgroupFileBytes+1))
	if err != nil || len(value) > maximumCgroupFileBytes {
		return nil, ErrUntrustedAccounting
	}
	return value, nil
}

func accountingDelta(baseline ResourceSnapshot, current ResourceSnapshot) (ResourceDelta, error) {
	if current.CPUUsage < baseline.CPUUsage ||
		current.OOMCount < baseline.OOMCount ||
		current.OOMKillCount < baseline.OOMKillCount ||
		current.CPUThrottledCount < baseline.CPUThrottledCount ||
		current.PIDLimitCount < baseline.PIDLimitCount ||
		current.MemoryCurrentBytes < 0 ||
		current.MemoryPeakBytes < current.MemoryCurrentBytes {
		return ResourceDelta{}, ErrUntrustedAccounting
	}
	return ResourceDelta{
		CPUUsage:           current.CPUUsage - baseline.CPUUsage,
		MemoryCurrentBytes: current.MemoryCurrentBytes,
		MemoryPeakBytes:    current.MemoryPeakBytes,
		OOMEvents:          current.OOMCount - baseline.OOMCount,
		OOMKills:           current.OOMKillCount - baseline.OOMKillCount,
		CPUThrottledEvents: current.CPUThrottledCount - baseline.CPUThrottledCount,
		PIDLimitEvents:     current.PIDLimitCount - baseline.PIDLimitCount,
	}, nil
}

func opaqueCgroupID(containerID string, relative string) string {
	digest := sha256.Sum256([]byte(containerID + "\x00" + relative))
	return fmt.Sprintf("cg-%x", digest[:12])
}
