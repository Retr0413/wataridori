package cli

import (
	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/core"
)

func newInventoryCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Inspect Cloud Run services and Wataridori management status",
	}
	cmd.AddCommand(newInventoryListCmd(g))
	return cmd
}

func newInventoryListCmd(g *globalFlags) *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Cloud Run services and mark managed/unmanaged services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needCloudRun: true})
			defer cleanup()
			if err != nil {
				return err
			}

			res, err := e.Inventory(cmd.Context(), core.InventoryRequest{Env: env})
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), res)
			}

			var rows [][]string
			for _, item := range res.Items {
				managed := "no"
				if item.Managed {
					managed = "yes"
				}
				rows = append(rows, []string{
					item.Env,
					item.Region,
					item.Service,
					managed,
					shortImage(item.DesiredImage),
					shortImage(item.ActualImage),
					orDash(item.Revision),
					string(item.State),
				})
			}
			table(cmd.OutOrStdout(), []string{
				"ENV", "REGION", "SERVICE", "MANAGED", "DESIRED", "ACTUAL", "REVISION", "STATE",
			}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "limit to one environment")
	return cmd
}
