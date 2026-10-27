package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// warnOutKey carries where a command's warnings go.
type warnOutKey struct{}

// setWarnOut sends cmd's warnings to its stderr (cmd.ErrOrStderr), so
// SetErr — a test's buffer, say — captures them as it does cobra's own.
func setWarnOut(cmd *cobra.Command, _ []string) {
	cmd.SetContext(context.WithValue(cmd.Context(), warnOutKey{}, cmd.ErrOrStderr()))
}

// warnf writes "warning: <msg>" to the command's stderr: what a script
// should know about a result that still printed (os.Stderr outside a
// command).
func warnf(ctx context.Context, format string, args ...any) {
	w, ok := ctx.Value(warnOutKey{}).(io.Writer)
	if !ok {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "warning: "+format+"\n", args...)
}
