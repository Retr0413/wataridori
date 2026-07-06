package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/core"
)

func newRollbackCmd(g *globalFlags) *cobra.Command {
	var (
		env      string
		service  string
		revision string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "rollback --env <env>",
		Short: "Route 100% of traffic back to the previous ready revision",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needCloudRun: true, needStore: true})
			defer cleanup()
			if err != nil {
				return err
			}

			plan, err := e.PlanRollback(cmd.Context(), core.RollbackRequest{Env: env, Service: service, Revision: revision})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			if !g.json {
				var rows [][]string
				for _, item := range plan.Items {
					rows = append(rows, []string{item.Service, item.CurrentRevision, "->", item.TargetRevision, shortImage(item.TargetImage)})
				}
				table(out, []string{"SERVICE", "CURRENT", "", "TARGET", "IMAGE"}, rows)
			}
			if !yes {
				ok, err := confirm(out, cmd.InOrStdin(), fmt.Sprintf("roll back %s?", plan.Env))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			res, err := e.ExecuteRollback(cmd.Context(), plan)
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(out, res)
			}
			for _, item := range res.Items {
				fmt.Fprintf(out, "%s: traffic switched to %s\n", item.Service, item.TargetRevision)
			}
			driftWarning(cmd, res.Env)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "target environment (required)")
	cmd.Flags().StringVar(&service, "service", "", "roll back only this service")
	cmd.Flags().StringVar(&revision, "revision", "", "explicit target revision (requires --service)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	_ = cmd.MarkFlagRequired("env")
	return cmd
}
