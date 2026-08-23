package cli

import (
	"fmt"
	"time"

	"github.com/Retr0413/wataridori/internal/core"
	"github.com/spf13/cobra"
)

func newEventCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "event", Short: "Accept provenance-bearing delivery events"}
	cmd.AddCommand(newImageEventCmd(g))
	return cmd
}

func newImageEventCmd(g *globalFlags) *cobra.Command {
	var env, service, image, repository, commit, workflowRun, occurredAt string
	var maxAge time.Duration
	cmd := &cobra.Command{
		Use:   "image --env <env> --service <service> --image <image@sha256:digest>",
		Short: "Verify an application artifact and update auto-policy desired state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			at := time.Now().UTC()
			if occurredAt != "" {
				parsed, err := time.Parse(time.RFC3339, occurredAt)
				if err != nil {
					return fmt.Errorf("invalid --occurred-at: %w", err)
				}
				at = parsed
			}
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needVerifier: true})
			defer cleanup()
			if err != nil {
				return err
			}
			res, err := e.RecordImageEvent(cmd.Context(), core.ImageEventRequest{
				Env: env, Service: service, Image: image, OccurredAt: at, MaxAge: maxAge,
				Provenance: core.Provenance{SourceRepository: repository, SourceCommit: commit, WorkflowRun: workflowRun},
			})
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), res)
			}
			state := "updated"
			if !res.Changed {
				state = "unchanged"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s from %s@%.12s (event %s)\n", state, res.Service, repository, commit, res.EventID)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "auto-policy environment (required)")
	cmd.Flags().StringVar(&service, "service", "", "logical service name (required)")
	cmd.Flags().StringVar(&image, "image", "", "digest-pinned image reference (required)")
	cmd.Flags().StringVar(&repository, "source-repository", "", "source owner/repository (required)")
	cmd.Flags().StringVar(&commit, "source-commit", "", "source full commit SHA (required)")
	cmd.Flags().StringVar(&workflowRun, "workflow-run", "", "source GitHub Actions run URL (required)")
	cmd.Flags().StringVar(&occurredAt, "occurred-at", "", "event time in RFC3339 (default: now)")
	cmd.Flags().DurationVar(&maxAge, "max-age", 24*time.Hour, "maximum accepted event age")
	for _, name := range []string{"env", "service", "image", "source-repository", "source-commit", "workflow-run"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}
