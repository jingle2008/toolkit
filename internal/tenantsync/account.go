/*
Package tenantsync records each tenancy's name, internal flag and account
status in the tenant metadata file, from the account admin service
(oci-ops account-admin-tool). It backs `toolkit tenant sync` and is
oc1 only.
*/
package tenantsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Account is what the sync reads for one tenancy.
type Account struct {
	TenancyID string
	Name      string
	Type      string // INTERNAL or EXTERNAL
	State     string // ACTIVE, DELETING, TERMINATED, ALWAYS_FREE_ONLY, ...
}

// Account types the service reports.
const (
	accountInternal = "INTERNAL"
	accountExternal = "EXTERNAL"
)

var (
	// ErrNotAuthenticated means the oci-ops session is missing or expired.
	// It stops a sync: every further lookup would fail the same way.
	ErrNotAuthenticated = errors.New("oci-ops is not authenticated or its session expired: re-authenticate (run any oci-ops command and accept the prompt), then rerun")
	// ErrRateLimited means the service throttled the call; it is retried.
	ErrRateLimited = errors.New("rate limited by the account admin service")
	// ErrNotFound means the service knows no such tenancy.
	ErrNotFound = errors.New("tenancy not found by the account admin service")
)

// Fetcher returns the raw get-account-details output for one tenancy.
type Fetcher interface {
	Fetch(ctx context.Context, ocid string) ([]byte, error)
}

type accountDetails struct {
	Data struct {
		AccountResponse struct {
			Account struct {
				Name         string `json:"name"`
				TenancyID    string `json:"tenancy-id"`
				AccountType  string `json:"account-type"`
				AccountState string `json:"account-state"`
			} `json:"account"`
		} `json:"account-response"`
	} `json:"data"`
}

// ParseAccount decodes get-account-details output. Only the first JSON
// value is read: oci-ops can print a session-expiry prompt after it.
func ParseAccount(raw []byte) (Account, error) {
	var d accountDetails
	if err := json.NewDecoder(strings.NewReader(string(raw))).Decode(&d); err != nil {
		return Account{}, fmt.Errorf("decode account details: %w", err)
	}
	a := d.Data.AccountResponse.Account
	if a.TenancyID == "" || a.Name == "" {
		return Account{}, errors.New("account details carry no tenancy id or name")
	}
	return Account{TenancyID: a.TenancyID, Name: a.Name, Type: a.AccountType, State: a.AccountState}, nil
}

// classify maps a failed oci-ops call to one of the sentinel errors when
// its output names the cause, else to a generic error.
func classify(stdout, stderr []byte, err error) error {
	out := string(stdout) + string(stderr)
	switch {
	case strings.Contains(out, "NotAuthenticated"), strings.Contains(out, "session has expired"):
		return ErrNotAuthenticated
	case strings.Contains(out, "TooManyRequests"):
		return ErrRateLimited
	case strings.Contains(out, "NotAuthorizedOrNotFound"):
		return ErrNotFound
	}
	msg := strings.TrimSpace(firstLine(string(stderr)))
	if err == nil {
		return fmt.Errorf("unexpected oci-ops output: %s", firstLine(string(stdout)))
	}
	return fmt.Errorf("oci-ops: %w: %s", err, msg)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
