package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/manifest"
)

func newPromoteCmd(g *globalFlags) *cobra.Command {
	var (
		from    string
		to      string
		service string
		yes     bool
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "promote --to <env> [--from <env>]",
		Short: "Copy image digests from one environment's manifests to another's and commit",
		Long: `Promote rewrites the target environment's manifests to the source
environment's image digests and records the change as a git commit.
It does not deploy: run "wataridori apply" (or let the Phase 2
controller reconcile) to roll the commit out.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{
				needRepo: true, needStore: !dryRun, needCopier: !dryRun, needCommit: !dryRun,
			})
			defer cleanup()
			if err != nil {
				return err
			}

			plan, err := e.PlanPromote(cmd.Context(), core.PromoteRequest{From: from, To: to, Service: service})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(plan.Items) == 0 {
				if g.json {
					return printJSON(out, plan)
				}
				fmt.Fprintf(out, "nothing to promote: %s already matches %s\n", plan.To, plan.From)
				return nil
			}
			if dryRun {
				if g.json {
					return printJSON(out, plan)
				}
				printPromotePlan(out, plan)
				fmt.Fprintln(out, "dry run: no registry, manifest, Git, or history changes made")
				return nil
			}

			if !g.json {
				printPromotePlan(out, plan)
			}
			if !yes {
				ok, err := confirm(out, cmd.InOrStdin(), fmt.Sprintf("promote %s -> %s?", plan.From, plan.To))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			res, err := e.ExecutePromote(cmd.Context(), plan)
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(out, res)
			}
			fmt.Fprintf(out, "committed %.12s — push it and run `wataridori apply --env %s` to deploy\n", res.CommitID, res.To)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source environment (default: the target's promoteFrom)")
	cmd.Flags().StringVar(&to, "to", "", "target environment (required)")
	cmd.Flags().StringVar(&service, "service", "", "promote only this service")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the promotion plan without copying, writing, or committing")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func printPromotePlan(out io.Writer, plan *core.PromotePlan) {
	var rows [][]string
	for _, item := range plan.Items {
		_, d, _ := manifest.SplitDigest(item.NewImage)
		copyNote := ""
		if item.NeedsCopy {
			copyNote = "copy image"
		}
		rows = append(rows, []string{item.Service, shortImage(item.OldImage), "->", manifest.ShortDigest(d), copyNote})
	}
	table(out, []string{"SERVICE", "CURRENT", "", "NEW DIGEST", ""}, rows)
}
