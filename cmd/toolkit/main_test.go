package main

import (
	"os"
	"testing"
)

func TestMain_HelpSmoke(t *testing.T) {
	t.Parallel()
	// Save and restore original args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"toolkit", "--help"}
	// main() will call cli.Execute which prints help and exits 0
	// We don't want to os.Exit, so just ensure it runs without panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main panicked: %v", r)
		}
	}()
	main()
}
