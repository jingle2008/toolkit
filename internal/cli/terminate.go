package cli

import (
	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation"
)

// terminateInstanceFn is the seam tests use to fake the OCI call.
var terminateInstanceFn = oci.TerminateInstance

func addTerminateCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		dryRun bool
		yes    bool
		ocid   string
	)
	cmd := &cobra.Command{
		Use:   "terminate <node>",
		Short: "Terminate the OCI instance backing a GPU node (destructive)",
		Long: `Submits a TerminateInstance request to OCI for the instance
backing <node>. PreserveBootVolume=false (same default as the TUI's
delete-node flow) — the boot volume is destroyed along with the
instance. Fire-and-forget: the request returns once OCI accepts it.

Destructive: requires explicit --yes; the interactive prompt is
deliberately disabled to prevent reflex "y" answers.

By default <node> is resolved against the live cluster (kube config +
env triple required). Pass --ocid to skip the cluster lookup when you
already know the instance OCID.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodeAction(cmd, cfgFile, mutation.TerminateNode, args[0], ocid, dryRun, yes, terminateInstanceFn)
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Look the target up, print what would happen and what was found, and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Required: this action has no interactive prompt")
	cmd.Flags().StringVar(&ocid, "ocid", "", "Skip k8s lookup and target this instance OCID directly")
	rootCmd.AddCommand(cmd)
}
