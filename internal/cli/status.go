package cli

import (
	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/core"
)

func newStatusCmd(g *globalFlags) *cobra.Command {
	var (
		env   string
		check bool
	)
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Compare desired state (Git) with actual state (Cloud Run)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needCloudRun: true})
			defer cleanup()
			if err != nil {
				return err
			}

			res, err := e.Status(cmd.Context(), core.StatusRequest{Env: env})
			if err != nil {
				return err
			}
			if g.json {
				if err := printJSON(cmd.OutOrStdout(), res); err != nil {
					return err
				}
			} else {
				var rows [][]string
				for _, s := range res.Services {
					mark := "✓"
					if s.State != core.StateInSync {
						mark = "✗"
					}
					rows = append(rows, []string{
						s.Env, s.Service, shortImage(s.DesiredImage), shortImage(s.ActualImage),
						orDash(s.Revision), mark + " " + string(s.State),
					})
				}
				table(cmd.OutOrStdout(), []string{"ENV", "SERVICE", "DESIRED", "ACTUAL", "REVISION", "STATUS"}, rows)
			}
			if check && res.Drift {
				return ErrDrift
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "limit to one environment")
	cmd.Flags().BoolVar(&check, "check", false, "exit with code 2 when any service drifts (for CI)")
	return cmd
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
