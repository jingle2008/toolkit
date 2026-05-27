//go:build !unix

package cli

// redirectStderr is a no-op on non-unix platforms. The fd-level
// redirection used on unix targets relies on dup2 syscall semantics
// that aren't portable to windows/plan9/etc. Anyone running the TUI
// on a non-unix host will see plugin stderr leak to their terminal —
// acceptable tradeoff for a TUI primarily targeting unix.
func redirectStderr(_ string) (func(), error) {
	return func() {}, nil
}
