package supervisor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	dockerStreamHeaderBytes = 8
	maximumDockerFrameBytes = 1 << 20
)

type boundedStream struct {
	bytes     []byte
	limit     int64
	observed  int64
	truncated bool
}

func newBoundedStream(limit int64) *boundedStream {
	return &boundedStream{bytes: make([]byte, 0, min(limit, 64<<10)), limit: limit}
}

func (stream *boundedStream) write(reader io.Reader, length int64) error {
	if length < 0 {
		return ErrSupervisorInfrastructure
	}
	stream.observed += length
	remaining := stream.limit - int64(len(stream.bytes))
	retain := min(max(remaining, 0), length)
	if retain != 0 {
		start := len(stream.bytes)
		stream.bytes = append(stream.bytes, make([]byte, retain)...)
		if _, err := io.ReadFull(reader, stream.bytes[start:]); err != nil {
			return err
		}
	}
	if discarded := length - retain; discarded != 0 {
		stream.truncated = true
		if _, err := io.CopyN(io.Discard, reader, discarded); err != nil {
			return err
		}
	}
	return nil
}

type attachOutput struct {
	stdout          []byte
	stderr          []byte
	stdoutTruncated bool
	stderrTruncated bool
	err             error
}

func drainDockerAttach(
	reader io.Reader,
	stdoutLimit int64,
	stderrLimit int64,
	overflow chan<- struct{},
) attachOutput {
	stdout := newBoundedStream(stdoutLimit)
	stderr := newBoundedStream(stderrLimit)
	header := make([]byte, dockerStreamHeaderBytes)
	overflowReported := false
	for {
		_, err := io.ReadFull(reader, header)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return attachOutput{err: fmt.Errorf("%w: read attach header: %v", ErrSupervisorInfrastructure, err)}
		}
		if header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return attachOutput{err: ErrSupervisorInfrastructure}
		}
		length := int64(binary.BigEndian.Uint32(header[4:]))
		if length > maximumDockerFrameBytes {
			return attachOutput{err: ErrSupervisorInfrastructure}
		}
		var target *boundedStream
		switch header[0] {
		case 1:
			target = stdout
		case 2:
			target = stderr
		default:
			return attachOutput{err: ErrSupervisorInfrastructure}
		}
		if err := target.write(reader, length); err != nil {
			return attachOutput{err: fmt.Errorf("%w: drain attach payload: %v", ErrSupervisorInfrastructure, err)}
		}
		if !overflowReported && (stdout.truncated || stderr.truncated) {
			overflowReported = true
			select {
			case overflow <- struct{}{}:
			default:
			}
		}
	}
	return attachOutput{
		stdout:          stdout.bytes,
		stderr:          stderr.bytes,
		stdoutTruncated: stdout.truncated,
		stderrTruncated: stderr.truncated,
	}
}
