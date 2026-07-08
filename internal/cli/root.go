package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/gitops"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/registry"
	"github.com/Retr0413/wataridori/internal/store"
)

// Version is stamped by goreleaser via ldflags.
var Version = "dev"

// ErrDrift makes main exit with code 2 (status --check).
var ErrDrift = errors.New("drift detected")

type globalFlags struct {
	repo string
	json bool
	db   string
}

// Execute runs the CLI; the returned error is already printed by cobra
// conventions disabled, so main decides rendering and exit codes.
func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	g := &globalFlags{}
	root := &cobra.Command{
		Use:           "wataridori",
		Short:         "GitOps CD for Cloud Run — digest-based promotion between environments",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&g.repo, "repo", "", "manifest repository root (default: search upward for wataridori.yaml)")
	root.PersistentFlags().BoolVar(&g.json, "json", false, "print results as JSON")
	root.PersistentFlags().StringVar(&g.db, "db", "", "history database path (default: $WATARIDORI_DB or ~/.local/share/wataridori/history.db)")

	root.AddCommand(
		newApplyCmd(g),
		newPromoteCmd(g),
		newRollbackCmd(g),
		newStatusCmd(g),
		newHistoryCmd(g),
		newServeCmd(g),
		newVersionCmd(),
	)
	return root
}

// loadRepo locates and loads the manifest repository, printing validation
// warnings to stderr.
func (g *globalFlags) loadRepo(errOut io.Writer) (*manifest.Repo, error) {
	dir := g.repo
	if dir == "" {
		dir = "."
	}
	root, err := manifest.FindRoot(dir)
	if err != nil {
		return nil, err
	}
	repo, warnings, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		fmt.Fprintln(errOut, "warning:", w)
	}
	return repo, nil
}

func (g *globalFlags) openStore() (*store.Store, error) {
	path := g.db
	if path == "" {
		var err error
		if path, err = store.DefaultPath(); err != nil {
			return nil, err
		}
	}
	return store.Open(path)
}

// engine assembles a core.Engine. Cloud Run and store connections are only
// opened when the command needs them.
type engineOptions struct {
	needRepo     bool
	needCloudRun bool
	needStore    bool
	needCopier   bool
	needCommit   bool
}

func (g *globalFlags) engine(cmd *cobra.Command, opts engineOptions) (*core.Engine, func(), error) {
	e := &core.Engine{Actor: resolveActor()}
	var closers []func()
	cleanup := func() {
		for _, c := range closers {
			c()
		}
	}

	if opts.needRepo {
		repo, err := g.loadRepo(cmd.ErrOrStderr())
		if err != nil {
			return nil, cleanup, err
		}
		e.Repo = repo
	}
	if opts.needCloudRun {
		client, err := cloudrun.NewClient(cmd.Context())
		if err != nil {
			return nil, cleanup, err
		}
		closers = append(closers, func() { _ = client.Close() })
		e.CloudRun = cloudRunAdapter{client}
	}
	if opts.needStore {
		st, err := g.openStore()
		if err != nil {
			return nil, cleanup, err
		}
		closers = append(closers, func() { _ = st.Close() })
		e.History = st
	}
	if opts.needCopier {
		e.Copier = registry.NewCopier()
	}
	if opts.needCommit {
		e.Commit = committerFunc(gitops.Commit)
	}
	return e, cleanup, nil
}

type committerFunc func(startDir string, files []string, message string) (string, error)

func (f committerFunc) Commit(startDir string, files []string, message string) (string, error) {
	return f(startDir, files, message)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the wataridori version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "wataridori", Version)
		},
	}
}
