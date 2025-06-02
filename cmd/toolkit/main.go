/*
Command toolkit is the CLI entry-point for the toolkit application.
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/app/toolkit"
	"github.com/jingle2008/toolkit/pkg/models"
	"k8s.io/client-go/util/homedir"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	category := toolkit.Tenant
	env := models.Environment{
		Type:   "preprod",
		Region: "us-chicago-1",
		Realm:  "oc1",
	}
	repoPath := "/Users/jinguzha/Work/repos/genai-shepherd-flocks"
	home := homedir.HomeDir()
	kubeConfig := filepath.Join(home, ".kube", "config")

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("WARN: closing debug log: %v\n", err)
		}
	}()

	model := toolkit.NewModel(ctx, repoPath, kubeConfig, env, category)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
