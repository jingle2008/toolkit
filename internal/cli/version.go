package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func addVersionCheckCommand(rootCmd *cobra.Command, currentVersion string) {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number and check for updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "toolkit version: %s\n", currentVersion)
			check, _ := cmd.Flags().GetBool("check-updates")
			if check {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				latest, err := fetchLatestRelease(ctx, &http.Client{Timeout: 5 * time.Second})
				if err != nil {
					return fmt.Errorf("failed to check latest version: %w", err)
				}
				if latest == currentVersion {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "You are running the latest version.")
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "A newer version is available: %s\n", latest)
				}
			}
			return nil
		},
	}
	versionCmd.Flags().Bool("check-updates", false, "Check for the latest release on GitHub")
	rootCmd.AddCommand(versionCmd)
}

func fetchLatestRelease(ctx context.Context, client *http.Client) (string, error) {
	const url = "https://api.github.com/repos/jingle2008/toolkit/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "toolkit")

	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			return
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Tag == "" {
		return "", errors.New("no tag_name in GitHub response")
	}
	return result.Tag, nil
}
