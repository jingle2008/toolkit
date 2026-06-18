package actions

import (
	"fmt"
	"os/exec"
	"runtime"
)

// execCommand is the seam tests use to capture the launched command
// without spawning a real browser.
var execCommand = exec.Command

// OpenURL opens url in the user's default browser using the
// platform-appropriate launcher. It starts the launcher without
// waiting for it to exit, returning an error only if the launcher
// process cannot be started.
func OpenURL(url string) error {
	var (
		name string
		args []string
	)
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default: // linux, *bsd, etc.
		name, args = "xdg-open", []string{url}
	}
	if err := execCommand(name, args...).Start(); err != nil {
		return fmt.Errorf("failed to open URL %q: %w", url, err)
	}
	return nil
}
