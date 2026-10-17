/*
Command tenant-sync records each oc1 tenancy's name, internal flag and
account status in toolkit's tenant metadata file, from the account admin
service via the oci-ops CLI. It is a dry run unless -write is passed. See
docs/USER_MANUAL.md.
*/
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/jingle2008/toolkit/internal/tenantsync"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := tenantsync.Main(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
