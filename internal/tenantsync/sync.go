package tenantsync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Options tunes the lookups.
type Options struct {
	Parallel int           // concurrent lookups; at least 1
	Retries  int           // retries of a rate-limited lookup
	Backoff  time.Duration // wait before the first retry; grows linearly
}

// Change is one tenancy's line in a Report. From and To are the old and
// new value of whatever the list is about; for Failed, To is the error.
type Change struct {
	ID, Tenant, From, To string
}

// Report lists what a sync changed or couldn't, by kind.
type Report struct {
	Looked            int
	Named             []Change // name filled in (From is empty)
	StatusChanged     []Change
	InternalSet       []Change // is_internal filled in
	NameConflicts     []Change // the service's name differs from yours, or is taken; yours kept
	InternalConflicts []Change // the service disagrees with your is_internal; yours kept
	Failed            []Change
}

// Sync looks up every tenancy in ids and applies the result to a copy of
// meta, which it returns with a report. Status is always refreshed; a
// name or internal flag is only filled in when missing, never
// overwritten, and a disagreement is reported instead. A failed lookup
// leaves the tenancy's entry as it was. It stops, returning
// ErrNotAuthenticated, as soon as a lookup finds the session gone.
func Sync(ctx context.Context, ids []string, meta *models.Metadata, f Fetcher, opts Options) (*models.Metadata, Report, error) {
	accounts, errs, err := fetchAll(ctx, ids, f, opts)
	if err != nil {
		return nil, Report{}, err
	}
	next := &models.Metadata{Tenants: slices.Clone(meta.Tenants)}
	report := Report{Looked: len(ids)}
	for _, id := range slices.Sorted(slices.Values(ids)) {
		if err := apply(next, id, accounts[id], errs[id], &report); err != nil {
			return nil, Report{}, err
		}
	}
	if err := next.Validate(); err != nil {
		return nil, Report{}, fmt.Errorf("synced metadata would be invalid: %w", err)
	}
	return next, report, nil
}

// fetchAll looks up every id, opts.Parallel at a time, retrying
// rate-limited calls. Per-tenancy failures are returned in errs; only
// ErrNotAuthenticated (or a cancelled ctx) fails the whole fetch.
func fetchAll(ctx context.Context, ids []string, f Fetcher, opts Options) (map[string]Account, map[string]error, error) {
	var mu sync.Mutex
	accounts := make(map[string]Account, len(ids))
	errs := make(map[string]error)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(max(opts.Parallel, 1))
	for _, id := range ids {
		g.Go(func() error {
			acct, err := fetchOne(gctx, id, f, opts)
			if errors.Is(err, ErrNotAuthenticated) || gctx.Err() != nil {
				return errors.Join(err, gctx.Err())
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[id] = err
			} else {
				accounts[id] = acct
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			return nil, nil, ErrNotAuthenticated
		}
		return nil, nil, err
	}
	return accounts, errs, nil
}

func fetchOne(ctx context.Context, id string, f Fetcher, opts Options) (Account, error) {
	for attempt := 0; ; attempt++ {
		raw, err := f.Fetch(ctx, id)
		if err == nil {
			acct, perr := ParseAccount(raw)
			if perr == nil && acct.TenancyID != id {
				perr = fmt.Errorf("response is for %s", acct.TenancyID)
			}
			return acct, perr
		}
		if !errors.Is(err, ErrRateLimited) || attempt >= opts.Retries {
			return Account{}, err
		}
		select {
		case <-ctx.Done():
			return Account{}, ctx.Err()
		case <-time.After(opts.Backoff * time.Duration(attempt+1)):
		}
	}
}

// apply folds one tenancy's lookup into next.
func apply(next *models.Metadata, id string, acct Account, lookupErr error, report *Report) error {
	entry := models.TenantMetadata{ID: id}
	if i := slices.IndexFunc(next.Tenants, func(t models.TenantMetadata) bool { return t.ID == id }); i >= 0 {
		entry = next.Tenants[i]
	}
	if lookupErr != nil {
		report.Failed = append(report.Failed, Change{ID: id, Tenant: nameOf(entry), To: lookupErr.Error()})
		return nil
	}

	switch {
	case entry.Name == nil || *entry.Name == "":
		if owner := nameOwner(next, acct.Name, id); owner != "" {
			report.NameConflicts = append(report.NameConflicts, Change{ID: id, Tenant: id, To: acct.Name + " (already used by " + owner + ")"})
			break
		}
		name := acct.Name
		entry.Name = &name
		report.Named = append(report.Named, Change{ID: id, Tenant: name, To: name})
	case *entry.Name != acct.Name:
		report.NameConflicts = append(report.NameConflicts, Change{ID: id, Tenant: *entry.Name, From: *entry.Name, To: acct.Name})
	}

	if acct.Type == accountInternal || acct.Type == accountExternal {
		internal := acct.Type == accountInternal
		switch {
		case entry.IsInternal == nil:
			entry.IsInternal = &internal
			report.InternalSet = append(report.InternalSet, Change{ID: id, Tenant: nameOf(entry), To: fmt.Sprint(internal)})
		case *entry.IsInternal != internal:
			report.InternalConflicts = append(report.InternalConflicts, Change{ID: id, Tenant: nameOf(entry), From: fmt.Sprint(*entry.IsInternal), To: fmt.Sprint(internal)})
		}
	}

	if entry.Status != acct.State {
		report.StatusChanged = append(report.StatusChanged, Change{ID: id, Tenant: nameOf(entry), From: entry.Status, To: acct.State})
		entry.Status = acct.State
	}
	return configloader.UpsertTenant(next, entry)
}

// nameOwner returns the ID of the tenancy other than id that already has
// name, or "".
func nameOwner(m *models.Metadata, name, id string) string {
	for _, t := range m.Tenants {
		if t.ID != id && t.Name != nil && *t.Name == name {
			return t.ID
		}
	}
	return ""
}

func nameOf(e models.TenantMetadata) string {
	if e.Name != nil && *e.Name != "" {
		return *e.Name
	}
	return e.ID
}
