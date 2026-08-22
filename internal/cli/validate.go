package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValidateCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate all manifests without contacting Git or Google Cloud",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true})
			defer cleanup()
			if err != nil {
				return err
			}
			result, err := e.ValidateManifests()
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "valid: %d environment(s), %d service manifest(s)\n", result.Environments, result.Services)
			return nil
		},
	}
}
