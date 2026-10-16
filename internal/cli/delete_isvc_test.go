//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
)

func stubPlanOrphan(outcome k8s.OrphanDeleteOutcome, err error) func() {
	return swap(&planOrphanISVCDeleteFn, func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		return outcome, err
	})
}

func TestDeleteISVCCmd_DryRunNamesThePlannedAction(t *testing.T) {
	for _, tc := range []struct {
		outcome k8s.OrphanDeleteOutcome
		want    string
	}{
		{k8s.OrphanDeleted, "DRY-RUN: would delete inference_service/ns-a/isvc-a"},
		{k8s.OrphanFinalized, "DRY-RUN: would force-delete inference_service/ns-a/isvc-a"},
	} {
		t.Run(string(tc.outcome), func(t *testing.T) {
			stageMutationEnv(t)
			var gotNS string
			defer stubResolveISVC(t, "ns-a", &gotNS)()
			defer stubPlanOrphan(tc.outcome, nil)()
			called := false
			defer swap(&deleteOrphanISVCFn, func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
				called = true
				return "", nil
			})()

			out, err := runRootCmd(t, []string{"delete", "isvc", "isvc-a", "--dry-run"}, "")
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if called {
				t.Fatal("--dry-run must not delete")
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("want %q, got: %q", tc.want, out)
			}
		})
	}
}

// A refusal (not an orphan) surfaces before any prompt, even on a dry run.
func TestDeleteISVCCmd_RefusalBeforePrompt(t *testing.T) {
	stageMutationEnv(t)
	var gotNS string
	defer stubResolveISVC(t, "ns-a", &gotNS)()
	defer stubPlanOrphan("", errors.New("not an orphaned InferenceService: its DAC dac-1 exists (ome.io); delete the DAC instead"))()
	defer swap(&deleteOrphanISVCFn, func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		t.Fatal("a refused target must not be deleted")
		return "", nil
	})()

	_, err := runRootCmd(t, []string{"delete", "isvc", "isvc-a", "--dry-run"}, "")
	if err == nil || !strings.Contains(err.Error(), "delete the DAC instead") {
		t.Fatalf("want refusal, got %v", err)
	}
}

func TestDeleteISVCCmd_RequiresYes(t *testing.T) {
	stageMutationEnv(t)
	var gotNS string
	defer stubResolveISVC(t, "ns-b", &gotNS)()
	defer stubPlanOrphan(k8s.OrphanDeleted, nil)()
	called := false
	defer swap(&deleteOrphanISVCFn, func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		called = true
		return "", nil
	})()

	_, err := runRootCmd(t, []string{"delete", "inference-service", "isvc-a"}, "y\n")
	if err == nil || !strings.Contains(err.Error(), "requires explicit --yes") {
		t.Fatalf("want explicit --yes error, got %v", err)
	}
	if called {
		t.Fatal("must not delete without --yes")
	}
}

func TestDeleteISVCCmd_YesDeletesAndSummarizes(t *testing.T) {
	stageMutationEnv(t)
	var gotNS string
	defer stubResolveISVC(t, "ns-a", &gotNS)()
	defer stubPlanOrphan(k8s.OrphanFinalized, nil)()
	var deletedNS, deletedName string
	defer swap(&deleteOrphanISVCFn, func(_ context.Context, _, _, namespace, name string) (k8s.OrphanDeleteOutcome, error) {
		deletedNS, deletedName = namespace, name
		return k8s.OrphanFinalized, nil
	})()

	out, err := runRootCmd(t, []string{"delete", "isvc", "isvc-a", "--namespace", "ns-x", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNS != "ns-x" || deletedNS != "ns-x" || deletedName != "isvc-a" {
		t.Errorf("resolved %q, deleted %s/%s; want ns-x/isvc-a", gotNS, deletedNS, deletedName)
	}
	if !strings.Contains(out, "force-deleted ns-x/isvc-a: finalizer removed") {
		t.Errorf("expected summary, got: %q", out)
	}
}
