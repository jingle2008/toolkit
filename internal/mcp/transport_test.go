package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// The guard passes well-formed messages on unchanged and answers every
// other non-blank line with a JSON-RPC error.
func TestGuardFrames(t *testing.T) {
	t.Parallel()
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	note := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	in := strings.Join([]string{
		req,
		`{not json`,
		``,
		`123`,
		`[]`,
		`[` + req + `]`,
		`{}`,
		`{"jsonrpc":"1.0","id":2,"method":"tools/list"}`,
		note + "\r",
		`"str"`,
	}, "\n") // the last line has no newline

	var pass, reply bytes.Buffer
	require.NoError(t, guardFrames(strings.NewReader(in), &pass, &reply))
	assert.Equal(t, req+"\n"+note+"\n", pass.String())

	replies := strings.Split(strings.TrimSpace(reply.String()), "\n")
	codes := make([]int64, 0, len(replies))
	for _, line := range replies {
		var r map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(line), &r), line)
		assert.JSONEq(t, `"2.0"`, string(r["jsonrpc"]))
		require.Contains(t, r, "id", "id must be present (null)")
		assert.Equal(t, "null", string(r["id"]))
		var e struct {
			Code int64 `json:"code"`
		}
		require.NoError(t, json.Unmarshal(r["error"], &e))
		codes = append(codes, e.Code)
	}
	assert.Equal(t, []int64{-32700, -32600, -32600, -32600, -32600, -32600, -32600}, codes)
}

// A line over the limit is answered and skipped; the next one still
// goes through.
func TestReadFrame_TooLong(t *testing.T) {
	t.Parallel()
	r := bufio.NewReaderSize(strings.NewReader(strings.Repeat("x", 100)+"\nok\n"), 16)
	line, tooLong, err := readFrame(r, 40)
	require.NoError(t, err)
	assert.True(t, tooLong)
	assert.Nil(t, line)
	line, tooLong, err = readFrame(r, 40)
	require.NoError(t, err)
	assert.False(t, tooLong)
	assert.Equal(t, "ok\n", string(line))
}

// End to end: a malformed line in the middle of a session is answered
// with an error and the server keeps serving the session.
func TestRun_SurvivesMalformedLine(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := NewServer(config.Config{RepoPath: "/dev/null", EnvType: "dev", EnvRegion: "us-ashburn-1", EnvRealm: "oc1"},
		stubLoader{}, logging.NewNoOpLogger(), "test")
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- srv.runIO(ctx, inR, outW) }()

	lines := bufio.NewScanner(outR)
	lines.Buffer(make([]byte, 1<<20), 1<<20)
	send := func(s string) {
		_, err := io.WriteString(inW, s+"\n")
		require.NoError(t, err)
	}
	next := func() map[string]any {
		require.True(t, lines.Scan(), "server closed its output")
		var m map[string]any
		require.NoError(t, json.Unmarshal(lines.Bytes(), &m))
		return m
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	assert.EqualValues(t, 1, next()["id"])
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	send(`{not json`)
	rej := next()
	assert.Nil(t, rej["id"])
	assert.EqualValues(t, -32700, rej["error"].(map[string]any)["code"]) //nolint:forcetypeassert // checked by the test

	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	resp := next()
	assert.EqualValues(t, 2, resp["id"])
	assert.Contains(t, resp, "result")

	require.NoError(t, inW.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("server didn't stop at the end of its input")
	}
}
