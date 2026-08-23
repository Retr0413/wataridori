package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Retr0413/wataridori/internal/core"
	"github.com/spf13/cobra"
)

func newPromotionCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "promotion", Short: "Inspect promotion eligibility"}
	cmd.AddCommand(newPromotionCheckCmd(g))
	return cmd
}

func newPromotionCheckCmd(g *globalFlags) *cobra.Command {
	var from, to, service, repository, commit, workflowRun, summaryFile string
	var paths []string
	var status, retries int
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "check --to <env> [--from <env>]",
		Short: "Produce read-only evidence for a production promotion candidate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needCloudRun: true, needVerifier: true, needHTTP: true})
			defer cleanup()
			if err != nil {
				return err
			}
			res, err := e.CheckPromotion(cmd.Context(), core.PromotionCheckRequest{
				From: from, To: to, Service: service, HTTPPaths: paths, HTTPStatus: status,
				HTTPTimeout: timeout, HTTPRetries: retries,
				Provenance: core.Provenance{SourceRepository: repository, SourceCommit: commit, WorkflowRun: workflowRun},
			})
			if err != nil {
				return err
			}
			markdown := promotionMarkdown(res)
			if summaryFile != "" {
				if err := os.WriteFile(summaryFile, []byte(markdown), 0o600); err != nil {
					return fmt.Errorf("writing promotion summary: %w", err)
				}
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), res)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), markdown)
			return err
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source environment (default: target promoteFrom)")
	cmd.Flags().StringVar(&to, "to", "", "target environment (required)")
	cmd.Flags().StringVar(&service, "service", "", "inspect one logical service")
	cmd.Flags().StringVar(&repository, "source-repository", "", "source owner/repository (required)")
	cmd.Flags().StringVar(&commit, "source-commit", "", "source full commit SHA (required)")
	cmd.Flags().StringVar(&workflowRun, "workflow-run", "", "source GitHub Actions run URL (required)")
	cmd.Flags().StringSliceVar(&paths, "http-path", nil, "Cloud Run relative health path (repeatable)")
	cmd.Flags().IntVar(&status, "http-status", 200, "expected HTTP status")
	cmd.Flags().DurationVar(&timeout, "http-timeout", 5*time.Second, "timeout for each HTTP attempt")
	cmd.Flags().IntVar(&retries, "http-retries", 3, "HTTP attempts (maximum 5)")
	cmd.Flags().StringVar(&summaryFile, "summary-file", "", "also write Markdown evidence to this file")
	for _, name := range []string{"to", "source-repository", "source-commit", "workflow-run"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}

func promotionMarkdown(e *core.PromotionEvidence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Wataridori promotion evidence\n\n- Evidence: `%s`\n- Route: `%s` → `%s`\n- Eligible: **%t**\n- Source: `%s@%s`\n- Workflow: %s\n", e.EvidenceID, e.From, e.To, e.Eligible, e.Provenance.SourceRepository, e.Provenance.SourceCommit, e.Provenance.WorkflowRun)
	for _, item := range e.Items {
		fmt.Fprintf(&b, "\n### `%s`\n\n| Check | Result | Detail |\n|---|---|---|\n", item.Service)
		for _, check := range item.Checks {
			result := "pass"
			if !check.Passed {
				result = "fail"
			}
			detail := strings.ReplaceAll(check.Detail, "|", "\\|")
			fmt.Fprintf(&b, "| %s | %s | %s |\n", check.Name, result, detail)
		}
	}
	return b.String()
}
