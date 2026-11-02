package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"

	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const (
	defaultMaximumArtifactFiles = 128
	defaultMaximumArtifactDepth = 8
	tarArchiveBlockBytes        = 512
)

var (
	ErrInvalidArtifact = errors.New("judge staging: invalid compiled artifact")
	ErrStagingQuota    = errors.New("judge staging: quota unavailable")
	ErrStagingIO       = errors.New("judge staging: trusted storage failure")
)

type StagerConfig struct {
	Root string

	GlobalBytes      int64
	GlobalInodes     int64
	PerAttemptBytes  int64
	PerAttemptInodes int64
	MinimumFreeBytes int64

	MaximumFiles int
	MaximumDepth int
}

type Stager struct {
	rootPath string
	root     *os.Root
	config   StagerConfig

	mu             sync.Mutex
	reservations   map[uuid.UUID]artifactReservation
	reservedBytes  int64
	reservedInodes int64
	closed         bool
}

type artifactReservation struct {
	bytes  int64
	inodes int64
}

type StageRequest struct {
	AttemptID        uuid.UUID
	Archive          []byte
	RuntimeProfile   problementity.RuntimeProfile
	ManifestChecksum string
}

type ArtifactFile struct {
	Path       string
	Size       int64
	Checksum   string
	Executable bool
}

type CompiledArtifact struct {
	AttemptID        uuid.UUID
	ManifestChecksum string
	ContentChecksum  string
	Files            []ArtifactFile
	RuntimeProfile   problementity.RuntimeProfile

	lease *artifactLease
}

type artifactLease struct {
	mu       sync.Mutex
	stager   *Stager
	root     string
	released bool
}

type artifactPlan struct {
	files       []plannedArtifact
	directories []string
	totalBytes  int64
	inodes      int64
	checksum    string
}

type plannedArtifact struct {
	path       string
	size       int64
	checksum   string
	executable bool
}

func NewStager(config StagerConfig) (*Stager, error) {
	if !filepath.IsAbs(config.Root) || filepath.Clean(config.Root) != config.Root ||
		config.GlobalBytes <= 0 || config.GlobalInodes <= 0 ||
		config.PerAttemptBytes <= 0 || config.PerAttemptInodes <= 0 ||
		config.PerAttemptBytes > config.GlobalBytes ||
		config.PerAttemptInodes > config.GlobalInodes ||
		config.MinimumFreeBytes < 0 {
		return nil, ErrStagingQuota
	}
	if config.MaximumFiles == 0 {
		config.MaximumFiles = defaultMaximumArtifactFiles
	}
	if config.MaximumDepth == 0 {
		config.MaximumDepth = defaultMaximumArtifactDepth
	}
	if config.MaximumFiles < 1 || config.MaximumFiles > 4096 ||
		config.MaximumDepth < 1 || config.MaximumDepth > 32 {
		return nil, ErrStagingQuota
	}
	if err := os.MkdirAll(config.Root, 0o700); err != nil {
		return nil, errors.Join(ErrStagingIO, err)
	}
	info, err := os.Lstat(config.Root)
	if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.Join(ErrStagingIO, err)
	}
	if err := os.Chmod(config.Root, 0o700); err != nil {
		return nil, errors.Join(ErrStagingIO, err)
	}
	root, err := os.OpenRoot(config.Root)
	if err != nil {
		return nil, errors.Join(ErrStagingIO, err)
	}
	return &Stager{
		rootPath:     config.Root,
		root:         root,
		config:       config,
		reservations: make(map[uuid.UUID]artifactReservation),
	}, nil
}

func (stager *Stager) Stage(
	ctx context.Context,
	request StageRequest,
) (CompiledArtifact, error) {
	if stager == nil || ctx == nil ||
		request.AttemptID == uuid.Nil || len(request.Archive) == 0 ||
		request.ManifestChecksum == "" ||
		request.ManifestChecksum != request.RuntimeProfile.ManifestChecksum {
		return CompiledArtifact{}, ErrInvalidArtifact
	}
	if err := ctx.Err(); err != nil {
		return CompiledArtifact{}, err
	}
	plan, err := inspectArtifactArchive(
		ctx,
		request.Archive,
		request.RuntimeProfile,
		stager.config.MaximumFiles,
		stager.config.MaximumDepth,
		stager.config.PerAttemptBytes,
		stager.config.PerAttemptInodes,
	)
	if err != nil {
		return CompiledArtifact{}, err
	}
	if err := stager.reserve(request.AttemptID, plan.totalBytes, plan.inodes); err != nil {
		return CompiledArtifact{}, err
	}
	reserved := true
	defer func() {
		if reserved {
			stager.releaseReservation(request.AttemptID)
		}
	}()

	stageID, err := uuid.NewV7()
	if err != nil {
		return CompiledArtifact{}, errors.Join(ErrStagingIO, err)
	}
	stageDirectory := ".stage-" + stageID.String()
	finalDirectory := request.AttemptID.String()
	if err := stager.root.Mkdir(stageDirectory, 0o700); err != nil {
		return CompiledArtifact{}, errors.Join(ErrStagingIO, err)
	}
	published := false
	defer func() {
		if !published {
			_ = stager.root.RemoveAll(stageDirectory)
		}
	}()
	if err := stager.writePlan(ctx, stageDirectory, request.Archive, plan); err != nil {
		return CompiledArtifact{}, err
	}
	if _, err := stager.root.Lstat(finalDirectory); err == nil ||
		!errors.Is(err, fs.ErrNotExist) {
		return CompiledArtifact{}, ErrInvalidArtifact
	}
	if err := syncDirectory(stager.root, stageDirectory); err != nil {
		return CompiledArtifact{}, err
	}
	if err := stager.root.Rename(stageDirectory, finalDirectory); err != nil {
		return CompiledArtifact{}, errors.Join(ErrStagingIO, err)
	}
	published = true
	if err := syncDirectory(stager.root, "."); err != nil {
		_ = stager.root.RemoveAll(finalDirectory)
		return CompiledArtifact{}, err
	}

	files := make([]ArtifactFile, 0, len(plan.files))
	for _, file := range plan.files {
		files = append(files, ArtifactFile{
			Path: file.path, Size: file.size, Checksum: file.checksum,
			Executable: file.executable,
		})
	}
	artifact := CompiledArtifact{
		AttemptID:        request.AttemptID,
		ManifestChecksum: request.ManifestChecksum,
		ContentChecksum:  plan.checksum,
		Files:            files,
		RuntimeProfile:   cloneArtifactRuntimeProfile(request.RuntimeProfile),
		lease:            &artifactLease{stager: stager, root: finalDirectory},
	}
	reserved = false
	return artifact, nil
}

func (stager *Stager) Close() error {
	if stager == nil {
		return nil
	}
	stager.mu.Lock()
	if stager.closed {
		stager.mu.Unlock()
		return nil
	}
	if len(stager.reservations) != 0 {
		stager.mu.Unlock()
		return ErrStagingQuota
	}
	stager.closed = true
	stager.mu.Unlock()
	return stager.root.Close()
}

func (artifact *CompiledArtifact) OpenFiles() ([]*os.File, error) {
	if artifact == nil || artifact.lease == nil ||
		len(artifact.Files) == 0 {
		return nil, ErrInvalidArtifact
	}
	artifact.lease.mu.Lock()
	defer artifact.lease.mu.Unlock()
	if artifact.lease.released || artifact.lease.stager == nil ||
		artifact.lease.root == "" {
		return nil, ErrInvalidArtifact
	}
	stager := artifact.lease.stager
	storageRoot := artifact.lease.root
	files := make([]*os.File, 0, len(artifact.Files))
	for _, item := range artifact.Files {
		file, err := stager.root.Open(path.Join(storageRoot, item.Path))
		if err != nil {
			closeFiles(files)
			return nil, errors.Join(ErrStagingIO, err)
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() != item.Size ||
			info.Mode().Perm() != 0o600 {
			_ = file.Close()
			closeFiles(files)
			return nil, ErrInvalidArtifact
		}
		digest := sha256.New()
		written, err := io.Copy(digest, file)
		if err != nil || written != item.Size ||
			fmt.Sprintf("sha256:%x", digest.Sum(nil)) != item.Checksum {
			_ = file.Close()
			closeFiles(files)
			return nil, ErrInvalidArtifact
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			_ = file.Close()
			closeFiles(files)
			return nil, errors.Join(ErrStagingIO, err)
		}
		files = append(files, file)
	}
	return files, nil
}

func (artifact *CompiledArtifact) Release(ctx context.Context) error {
	if artifact == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidArtifact
	}
	if artifact.lease == nil {
		return nil
	}
	artifact.lease.mu.Lock()
	defer artifact.lease.mu.Unlock()
	if artifact.lease.released {
		return nil
	}
	if artifact.lease.stager == nil || artifact.lease.root == "" {
		return ErrInvalidArtifact
	}
	stager := artifact.lease.stager
	if err := stager.root.RemoveAll(artifact.lease.root); err != nil {
		return errors.Join(ErrStagingIO, err)
	}
	if err := syncDirectory(stager.root, "."); err != nil {
		return err
	}
	stager.releaseReservation(artifact.AttemptID)
	artifact.lease.released = true
	artifact.lease.stager = nil
	artifact.lease.root = ""
	return nil
}

func (stager *Stager) reserve(attemptID uuid.UUID, bytes int64, inodes int64) error {
	stager.mu.Lock()
	defer stager.mu.Unlock()
	if stager.closed || bytes < 0 || inodes < 1 ||
		bytes > stager.config.PerAttemptBytes ||
		inodes > stager.config.PerAttemptInodes ||
		stager.reservedBytes > stager.config.GlobalBytes-bytes ||
		stager.reservedInodes > stager.config.GlobalInodes-inodes {
		return ErrStagingQuota
	}
	if _, duplicate := stager.reservations[attemptID]; duplicate {
		return ErrStagingQuota
	}
	free, err := availableBytes(stager.rootPath)
	if err != nil || free < stager.config.MinimumFreeBytes ||
		bytes > free-stager.config.MinimumFreeBytes {
		return errors.Join(ErrStagingQuota, err)
	}
	stager.reservations[attemptID] = artifactReservation{bytes: bytes, inodes: inodes}
	stager.reservedBytes += bytes
	stager.reservedInodes += inodes
	return nil
}

func (stager *Stager) releaseReservation(attemptID uuid.UUID) {
	stager.mu.Lock()
	reservation, exists := stager.reservations[attemptID]
	if exists {
		delete(stager.reservations, attemptID)
		stager.reservedBytes -= reservation.bytes
		stager.reservedInodes -= reservation.inodes
	}
	stager.mu.Unlock()
}

func (stager *Stager) writePlan(
	ctx context.Context,
	stageDirectory string,
	archiveBytes []byte,
	plan artifactPlan,
) error {
	for _, directory := range plan.directories {
		if err := stager.root.MkdirAll(path.Join(stageDirectory, directory), 0o700); err != nil {
			return errors.Join(ErrStagingIO, err)
		}
	}
	reader := tar.NewReader(bytes.NewReader(archiveBytes))
	plannedByPath := make(map[string]plannedArtifact, len(plan.files))
	for _, planned := range plan.files {
		plannedByPath[planned.path] = planned
	}
	writtenPaths := make(map[string]struct{}, len(plan.files))
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Join(ErrInvalidArtifact, err)
		}
		relative, isFile, err := normalizeArtifactHeader(header)
		if err != nil {
			return err
		}
		if !isFile {
			continue
		}
		planned, exists := plannedByPath[relative]
		if !exists || planned.size != header.Size {
			return ErrInvalidArtifact
		}
		if _, duplicate := writtenPaths[relative]; duplicate {
			return ErrInvalidArtifact
		}
		writtenPaths[relative] = struct{}{}
		directory := path.Dir(relative)
		temporaryID, err := uuid.NewV7()
		if err != nil {
			return errors.Join(ErrStagingIO, err)
		}
		temporaryName := path.Join(stageDirectory, directory, ".tmp-"+temporaryID.String())
		finalName := path.Join(stageDirectory, relative)
		file, err := stager.root.OpenFile(
			temporaryName,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW,
			0o600,
		)
		if err != nil {
			return errors.Join(ErrStagingIO, err)
		}
		digest := sha256.New()
		written, copyErr := copyNWithContext(
			ctx,
			io.MultiWriter(file, digest),
			reader,
			header.Size,
		)
		syncErr := file.Sync()
		closeErr := file.Close()
		if copyErr != nil || written != header.Size || syncErr != nil || closeErr != nil {
			_ = stager.root.Remove(temporaryName)
			if copyErr != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.Join(ErrStagingIO, copyErr, syncErr, closeErr)
		}
		checksum := fmt.Sprintf("sha256:%x", digest.Sum(nil))
		if checksum != planned.checksum {
			_ = stager.root.Remove(temporaryName)
			return ErrInvalidArtifact
		}
		if _, err := stager.root.Lstat(finalName); err == nil ||
			!errors.Is(err, fs.ErrNotExist) {
			_ = stager.root.Remove(temporaryName)
			return ErrInvalidArtifact
		}
		if err := stager.root.Rename(temporaryName, finalName); err != nil {
			_ = stager.root.Remove(temporaryName)
			return errors.Join(ErrStagingIO, err)
		}
	}
	if len(writtenPaths) != len(plan.files) {
		return ErrInvalidArtifact
	}
	return nil
}

func inspectArtifactArchive(
	ctx context.Context,
	archiveBytes []byte,
	profile problementity.RuntimeProfile,
	maximumFiles int,
	maximumDepth int,
	maximumBytes int64,
	maximumInodes int64,
) (artifactPlan, error) {
	if ctx == nil || len(profile.Artifacts) == 0 || profile.ManifestChecksum == "" ||
		len(archiveBytes) < 2*tarArchiveBlockBytes || len(archiveBytes)%tarArchiveBlockBytes != 0 {
		return artifactPlan{}, ErrInvalidArtifact
	}
	reader := tar.NewReader(bytes.NewReader(archiveBytes))
	seen := make(map[string]bool)
	directories := make(map[string]struct{})
	files := make([]plannedArtifact, 0, len(profile.Artifacts))
	matchCounts := make([]int, len(profile.Artifacts))
	var totalBytes int64
	for {
		if err := ctx.Err(); err != nil {
			return artifactPlan{}, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return artifactPlan{}, errors.Join(ErrInvalidArtifact, err)
		}
		relative, isFile, err := normalizeArtifactHeader(header)
		if err != nil {
			return artifactPlan{}, err
		}
		if relative == "" {
			continue
		}
		if _, duplicate := seen[relative]; duplicate {
			return artifactPlan{}, ErrInvalidArtifact
		}
		for parent := path.Dir(relative); parent != "."; parent = path.Dir(parent) {
			if parentIsFile, exists := seen[parent]; exists && parentIsFile {
				return artifactPlan{}, ErrInvalidArtifact
			}
		}
		if isFile {
			for existing := range seen {
				if strings.HasPrefix(existing, relative+"/") {
					return artifactPlan{}, ErrInvalidArtifact
				}
			}
		}
		seen[relative] = isFile
		if pathDepth(relative) > maximumDepth {
			return artifactPlan{}, ErrInvalidArtifact
		}
		if !isFile {
			directories[relative] = struct{}{}
			continue
		}
		if len(files) >= maximumFiles || header.Size < 0 ||
			header.Size > maximumBytes-totalBytes {
			return artifactPlan{}, ErrInvalidArtifact
		}
		ruleIndex, executable, allowed := matchArtifactRule(profile.Artifacts, relative, header.Size)
		if !allowed {
			return artifactPlan{}, ErrInvalidArtifact
		}
		matchCounts[ruleIndex]++
		maximumMatches := profile.Artifacts[ruleIndex].MaximumFiles
		if maximumMatches == 0 {
			maximumMatches = 1
		}
		if matchCounts[ruleIndex] > maximumMatches {
			return artifactPlan{}, ErrInvalidArtifact
		}
		digest := sha256.New()
		written, err := copyNWithContext(ctx, digest, reader, header.Size)
		if err != nil || written != header.Size {
			if ctx.Err() != nil {
				return artifactPlan{}, ctx.Err()
			}
			return artifactPlan{}, ErrInvalidArtifact
		}
		totalBytes += header.Size
		files = append(files, plannedArtifact{
			path: relative, size: header.Size,
			checksum:   fmt.Sprintf("sha256:%x", digest.Sum(nil)),
			executable: executable,
		})
		for directory := path.Dir(relative); directory != "."; directory = path.Dir(directory) {
			directories[directory] = struct{}{}
		}
	}
	for index, count := range matchCounts {
		if count == 0 && profile.Artifacts[index].Required {
			return artifactPlan{}, ErrInvalidArtifact
		}
	}
	if len(files) == 0 {
		return artifactPlan{}, ErrInvalidArtifact
	}
	inodes := int64(len(files) + len(directories) + 1)
	if inodes > maximumInodes {
		return artifactPlan{}, ErrInvalidArtifact
	}
	slices.SortFunc(files, func(left plannedArtifact, right plannedArtifact) int {
		return strings.Compare(left.path, right.path)
	})
	directoryNames := make([]string, 0, len(directories))
	for directory := range directories {
		directoryNames = append(directoryNames, directory)
	}
	slices.Sort(directoryNames)
	return artifactPlan{
		files:       files,
		directories: directoryNames,
		totalBytes:  totalBytes,
		inodes:      inodes,
		checksum:    compiledArtifactChecksum(profile.ManifestChecksum, files),
	}, nil
}

func copyNWithContext(
	ctx context.Context,
	destination io.Writer,
	source io.Reader,
	size int64,
) (int64, error) {
	if ctx == nil || destination == nil || source == nil || size < 0 {
		return 0, ErrInvalidArtifact
	}
	limited := &io.LimitedReader{
		R: &artifactContextReader{ctx: ctx, reader: source},
		N: size,
	}
	written, err := io.CopyBuffer(destination, limited, make([]byte, 32<<10))
	if err != nil {
		return written, err
	}
	if written != size {
		return written, io.ErrUnexpectedEOF
	}
	return written, nil
}

type artifactContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *artifactContextReader) Read(destination []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(destination)
}

func normalizeArtifactHeader(header *tar.Header) (string, bool, error) {
	if header == nil || header.Name == "" || len(header.Name) > 512 ||
		!utf8.ValidString(header.Name) ||
		strings.ContainsAny(header.Name, "\\\x00") || containsUnsafePathRune(header.Name) ||
		path.IsAbs(header.Name) || path.Clean(header.Name) != strings.TrimSuffix(header.Name, "/") ||
		len(header.PAXRecords) != 0 ||
		header.Uid != 65532 || header.Gid != 65532 {
		return "", false, ErrInvalidArtifact
	}
	clean := strings.TrimSuffix(header.Name, "/")
	if clean == "artifact" {
		if header.Typeflag != tar.TypeDir || header.Size != 0 {
			return "", false, ErrInvalidArtifact
		}
		return "", false, nil
	}
	if !strings.HasPrefix(clean, "artifact/") {
		return "", false, ErrInvalidArtifact
	}
	relative := strings.TrimPrefix(clean, "artifact/")
	if relative == "" || relative == "." || strings.HasPrefix(relative, "../") ||
		path.Clean(relative) != relative {
		return "", false, ErrInvalidArtifact
	}
	for _, component := range strings.Split(relative, "/") {
		if component == "" || component == "." || component == ".." ||
			len(component) > 255 || strings.HasPrefix(component, ".wh.") {
			return "", false, ErrInvalidArtifact
		}
	}
	switch header.Typeflag {
	case tar.TypeDir:
		if header.Size != 0 || header.Mode&(0o4000|0o2000|0o1000) != 0 {
			return "", false, ErrInvalidArtifact
		}
		return relative, false, nil
	case tar.TypeReg:
		if header.Linkname != "" ||
			header.Mode&(0o4000|0o2000|0o1000) != 0 {
			return "", false, ErrInvalidArtifact
		}
		return relative, true, nil
	default:
		return "", false, ErrInvalidArtifact
	}
}

func matchArtifactRule(
	rules []problementity.RuntimeArtifact,
	name string,
	size int64,
) (int, bool, bool) {
	matchedIndex := -1
	for index, rule := range rules {
		matched, err := path.Match(rule.Path, name)
		if err != nil || !matched {
			continue
		}
		if matchedIndex != -1 || size > rule.MaximumBytes {
			return 0, false, false
		}
		matchedIndex = index
	}
	if matchedIndex == -1 {
		return 0, false, false
	}
	return matchedIndex, rules[matchedIndex].Executable, true
}

func compiledArtifactChecksum(manifestChecksum string, files []plannedArtifact) string {
	digest := sha256.New()
	writeArtifactPart(digest, "judgify.compiled-artifact.v1")
	writeArtifactPart(digest, manifestChecksum)
	for _, file := range files {
		writeArtifactPart(digest, file.path)
		writeArtifactPart(digest, fmt.Sprintf("%d", file.size))
		writeArtifactPart(digest, file.checksum)
		if file.executable {
			writeArtifactPart(digest, "executable")
		} else {
			writeArtifactPart(digest, "data")
		}
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil))
}

func writeArtifactPart(digest hash.Hash, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}

func availableBytes(root string) (int64, error) {
	var statistics unix.Statfs_t
	if err := unix.Statfs(root, &statistics); err != nil {
		return 0, err
	}
	blockSize := int64(statistics.Bsize)
	available := uint64(statistics.Bavail)
	if blockSize <= 0 || available > uint64(^uint64(0)>>1)/uint64(blockSize) {
		return 0, ErrStagingQuota
	}
	return int64(available * uint64(blockSize)), nil
}

func syncDirectory(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return errors.Join(ErrStagingIO, err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return errors.Join(ErrStagingIO, syncErr, closeErr)
	}
	return nil
}

func pathDepth(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "/") + 1
}

func containsUnsafePathRune(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return true
		}
	}
	return false
}

func closeFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func cloneArtifactRuntimeProfile(
	profile problementity.RuntimeProfile,
) problementity.RuntimeProfile {
	profile.Compile.Arguments = slices.Clone(profile.Compile.Arguments)
	profile.Compile.Environment = slices.Clone(profile.Compile.Environment)
	profile.Run.Arguments = slices.Clone(profile.Run.Arguments)
	profile.Run.Environment = slices.Clone(profile.Run.Environment)
	profile.Artifacts = slices.Clone(profile.Artifacts)
	profile.Workspace.Values = slices.Clone(profile.Workspace.Values)
	profile.Workspace.TrustedFiles = slices.Clone(profile.Workspace.TrustedFiles)
	return profile
}
