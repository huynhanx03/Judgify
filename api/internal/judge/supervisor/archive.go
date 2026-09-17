package supervisor

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"time"
)

func (supervisor *SandboxSupervisor) putInputArchive(
	ctx context.Context,
	containerID string,
	files []InputFile,
	createArtifactDirectory bool,
) error {
	if len(files) == 0 && !createArtifactDirectory {
		return nil
	}
	reader, writer := io.Pipe()
	writeResult := make(chan error, 1)
	go func() {
		err := writeCanonicalInputArchive(writer, files, createArtifactDirectory)
		_ = writer.CloseWithError(err)
		writeResult <- err
	}()
	copyErr := supervisor.engine.PutArchive(ctx, containerID, "/workspace", reader)
	_ = reader.CloseWithError(copyErr)
	writeErr := <-writeResult
	if copyErr != nil || writeErr != nil {
		return fmt.Errorf(
			"%w: copy input archive: %v",
			ErrSupervisorInfrastructure,
			errors.Join(copyErr, writeErr),
		)
	}
	return nil
}

func writeCanonicalInputArchive(
	destination io.Writer,
	files []InputFile,
	createArtifactDirectory bool,
) error {
	archive := tar.NewWriter(destination)
	directories := make(map[string]struct{})
	if createArtifactDirectory {
		directories["artifact"] = struct{}{}
	}
	for _, file := range files {
		for directory := path.Dir(file.Path); directory != "."; directory = path.Dir(directory) {
			directories[directory] = struct{}{}
			if path.Dir(directory) == directory {
				break
			}
		}
	}
	directoryNames := make([]string, 0, len(directories))
	for directory := range directories {
		directoryNames = append(directoryNames, directory)
	}
	slices.Sort(directoryNames)
	epoch := time.Unix(0, 0).UTC()
	for _, directory := range directoryNames {
		header := &tar.Header{
			Name: directory + "/", Typeflag: tar.TypeDir, Mode: 0o700,
			Uid: 65532, Gid: 65532, ModTime: epoch, AccessTime: epoch,
			ChangeTime: epoch, Format: tar.FormatPAX,
		}
		if err := archive.WriteHeader(header); err != nil {
			_ = archive.Close()
			return err
		}
	}
	for _, file := range files {
		header := &tar.Header{
			Name: file.Path, Typeflag: tar.TypeReg, Mode: int64(file.Mode.Perm()),
			Size: file.Size, Uid: 65532, Gid: 65532, ModTime: epoch,
			AccessTime: epoch, ChangeTime: epoch, Format: tar.FormatPAX,
		}
		if err := archive.WriteHeader(header); err != nil {
			_ = archive.Close()
			return err
		}
		written, err := io.CopyN(archive, file.Reader, file.Size)
		if err != nil || written != file.Size {
			_ = archive.Close()
			return fmt.Errorf("input file %s has inconsistent length", safePathLabel(file.Path))
		}
		var extra [1]byte
		if count, err := file.Reader.Read(extra[:]); count != 0 || (err != nil && !errors.Is(err, io.EOF)) {
			_ = archive.Close()
			return fmt.Errorf("input file %s exceeds declared length", safePathLabel(file.Path))
		}
	}
	return archive.Close()
}

func safePathLabel(value string) string {
	if value == "" {
		return "invalid"
	}
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}
