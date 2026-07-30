package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/core"
)

func newApplyCmd(g *globalFlags) *cobra.Command {
	var (
		env     string
		service string
		dryRun  bool
		force   bool
		timeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "apply --env <env>",
		Short: "Deploy an environment's manifests to Cloud Run",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needCloudRun: true, needStore: !dryRun})
			defer cleanup()
			if err != nil {
				return err
			}

			res, err := e.Apply(cmd.Context(), core.ApplyRequest{
				Env: env, Service: service, DryRun: dryRun, Force: force, Timeout: timeout,
			})
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), res)
			}

			var rows [][]string
			for _, s := range res.Services {
				status := "deployed"
				if res.DryRun {
					if s.InSync {
						status = "up to date"
					} else if s.ActualImage == "" {
						status = "would create"
					} else {
						status = "would update"
					}
				}
				rows = append(rows, []string{s.Service, shortImage(s.DesiredImage), s.Revision, status})
			}
			table(cmd.OutOrStdout(), []string{"SERVICE", "IMAGE", "REVISION", "STATUS"}, rows)
			for _, s := range res.Services {
				if len(s.Unmanaged) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: %s has settings the manifest cannot express; applying removes them: %s\n",
						s.Service, strings.Join(s.Unmanaged, ", "))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "target environment (required)")
	cmd.Flags().StringVar(&service, "service", "", "deploy only this service")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the diff without deploying")
	cmd.Flags().BoolVar(&force, "force", false, "apply even when it removes settings the manifest cannot express")
	cmd.Flags().DurationVar(&timeout, "timeout", core.DefaultApplyTimeout, "wait for the revision to become ready")
	_ = cmd.MarkFlagRequired("env")
	return cmd
}

// driftWarning is shared by rollback to remind that the manifest lags.
func driftWarning(cmd *cobra.Command, env string) {
	fmt.Fprintf(cmd.ErrOrStderr(),
		"note: the manifest still points at the previous digest; update it (or promote again) to make this permanent. `wataridori status --env %s` will show drift until then.\n", env)
}
