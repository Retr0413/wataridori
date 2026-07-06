package cli

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/manifest"
)

func newHistoryCmd(g *globalFlags) *cobra.Command {
	var (
		env   string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show recorded apply/promote/rollback operations, newest first",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, cleanup, err := g.engine(cmd, engineOptions{needStore: true})
			defer cleanup()
			if err != nil {
				return err
			}

			res, err := e.ListHistory(cmd.Context(), core.HistoryRequest{Env: env, Limit: limit})
			if err != nil {
				return err
			}
			if g.json {
				return printJSON(cmd.OutOrStdout(), res)
			}

			var rows [][]string
			for _, entry := range res.Entries {
				detail := ""
				if len(entry.Detail) > 0 {
					if b, err := json.Marshal(entry.Detail); err == nil {
						detail = string(b)
					}
				}
				rows = append(rows, []string{
					entry.Time.Local().Format(time.DateTime),
					entry.Actor,
					string(entry.Action),
					entry.Env,
					entry.Service,
					manifest.ShortDigest(entry.Digest),
					detail,
				})
			}
			table(cmd.OutOrStdout(), []string{"TIME", "ACTOR", "ACTION", "ENV", "SERVICE", "DIGEST", "DETAIL"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "filter by environment")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum entries to show")
	return cmd
}
