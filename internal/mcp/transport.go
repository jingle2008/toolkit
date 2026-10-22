package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxFrameBytes bounds one inbound line, as the SDK's own
// DefaultMaxLineLength does.
const maxFrameBytes = sdk.DefaultMaxLineLength

// newGuardedTransport is the stdio transport with a guard in front of the
// SDK's reader. The SDK decodes stdin as one JSON stream and ends the
// session on the first frame it can't decode: one malformed line from a
// client kills the server, and the replies still in flight with it. The
// guard reads in line by line instead and passes on only well-formed
// JSON-RPC messages; it answers anything else itself, with the JSON-RPC
// error the spec gives (-32700 for what isn't JSON, -32600 for JSON that
// isn't a message), and goes on reading.
//
// Batches (JSON arrays) are answered with -32600 too: MCP dropped them in
// 2025-06-18, and the SDK ends the session on one under that version.
//
// Both the guard and the SDK write to out, through one lock; the SDK
// writes each message in one Write, so replies never interleave.
func newGuardedTransport(in io.Reader, out io.Writer) sdk.Transport {
	w := &lockedWriter{w: out}
	pr, pw := io.Pipe()
	go func() { _ = pw.CloseWithError(guardFrames(in, pw, w)) }()
	return &sdk.IOTransport{Reader: pr, Writer: nopWriteCloser{w}, MaxLineLength: -1}
}

// guardFrames copies each well-formed JSON-RPC message line from in to
// pass, and answers every other non-blank line on reply. It returns nil
// at the end of in, or the error that stopped it.
func guardFrames(in io.Reader, pass, reply io.Writer) error {
	r := bufio.NewReaderSize(in, 64*1024)
	for {
		line, tooLong, err := readFrame(r, maxFrameBytes)
		if len(bytes.TrimSpace(line)) > 0 || tooLong {
			var rej *jsonrpc.Error
			if tooLong {
				rej = &jsonrpc.Error{Code: jsonrpc.CodeInvalidRequest, Message: fmt.Sprintf("message exceeds %d bytes", maxFrameBytes)}
			} else {
				rej = checkFrame(line)
			}
			if rej == nil {
				if _, werr := pass.Write(append(bytes.TrimRight(line, "\r\n"), '\n')); werr != nil {
					return werr
				}
			} else if werr := writeRejection(reply, rej); werr != nil {
				return werr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// readFrame reads one line (through its newline, or to the end of r). A
// line longer than limit is discarded to its end and reported tooLong.
func readFrame(r *bufio.Reader, limit int) (line []byte, tooLong bool, err error) {
	for {
		chunk, err := r.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > limit {
				tooLong, line = true, nil
			} else {
				line = append(line, chunk...)
			}
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, tooLong, err
		}
	}
}

// checkFrame returns the error to answer line with, or nil when it is a
// JSON-RPC message the SDK can take.
func checkFrame(line []byte) *jsonrpc.Error {
	trimmed := bytes.TrimSpace(line)
	if !json.Valid(trimmed) {
		return &jsonrpc.Error{Code: jsonrpc.CodeParseError, Message: "parse error: not valid JSON"}
	}
	if trimmed[0] != '{' {
		return &jsonrpc.Error{Code: jsonrpc.CodeInvalidRequest, Message: "invalid request: not a JSON-RPC message object (batches are not supported)"}
	}
	if _, err := jsonrpc.DecodeMessage(trimmed); err != nil {
		return &jsonrpc.Error{Code: jsonrpc.CodeInvalidRequest, Message: "invalid request: " + err.Error()}
	}
	return nil
}

// writeRejection answers a frame that couldn't be read. Its id is null:
// the spec's id when the request's own can't be determined.
func writeRejection(w io.Writer, rej *jsonrpc.Error) error {
	data, err := json.Marshal(struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      any            `json:"id"`
		Error   *jsonrpc.Error `json:"error"`
	}{"2.0", nil, rej})
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// lockedWriter serializes writes from the guard and the SDK.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// nopWriteCloser is an io.WriteCloser whose Close does nothing: closing
// the session mustn't close stdout.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
