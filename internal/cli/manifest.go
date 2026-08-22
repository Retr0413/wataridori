package cli

import (
	"fmt"

	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/spf13/cobra"
)

func newManifestCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Update Git desired-state manifests",
	}
	cmd.AddCommand(newManifestSetImageCmd(g))
	return cmd
}

func newManifestSetImageCmd(g *globalFlags) *cobra.Command {
	var env, service, image, requirePolicy string
	cmd := &cobra.Command{
		Use:   "set-image --env <env> --service <service> --image <image@sha256:digest>",
		Short: "Set one service to an externally built immutable image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true})
			defer cleanup()
			if err != nil {
				return err
			}
			result, err := e.SetManifestImage(core.SetManifestImageRequest{
				Env: env, Service: service, Image: image, RequirePolicy: manifest.Policy(requirePolicy),
			})
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), result)
			}
			if !result.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "unchanged: %s already uses %s\n", result.Service, shortImage(result.NewImage))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s: %s -> %s (%s)\n", result.Service, shortImage(result.OldImage), shortImage(result.NewImage), result.File)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "manifest environment to update (required)")
	cmd.Flags().StringVar(&service, "service", "", "logical service name to update (required)")
	cmd.Flags().StringVar(&image, "image", "", "digest-pinned image reference (required)")
	cmd.Flags().StringVar(&requirePolicy, "require-policy", "", "refuse unless the environment has this policy (auto or manual)")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("service")
	_ = cmd.MarkFlagRequired("image")
	return cmd
}
