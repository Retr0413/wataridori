package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/controller"
	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/gitops"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/registry"
	"github.com/Retr0413/wataridori/internal/server"
	"github.com/Retr0413/wataridori/web"
)

func newServeCmd(g *globalFlags) *cobra.Command {
	var (
		addr        string
		reconcile   bool
		reconcileIv time.Duration
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Connect RPC server (Phase 2 controller/UI backend)",
		Long: "Serve the same use cases as the CLI over Connect RPC. The Cloud Run\n" +
			"client and history store are opened once; the manifest repo is reloaded\n" +
			"per request so it stays in step with Git.\n\n" +
			"With --reconcile, a control loop periodically applies every policy:auto\n" +
			"environment and POST /-/reconcile triggers an immediate cycle.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, g, serveOptions{addr: addr, reconcile: reconcile, interval: reconcileIv})
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	cmd.Flags().BoolVar(&reconcile, "reconcile", false, "run the reconcile loop for policy:auto environments")
	cmd.Flags().DurationVar(&reconcileIv, "reconcile-interval", 60*time.Second, "reconcile loop interval")
	return cmd
}

type serveOptions struct {
	addr      string
	reconcile bool
	interval  time.Duration
}

func runServe(cmd *cobra.Command, g *globalFlags, opts serveOptions) error {
	ctx := cmd.Context()

	// Long-lived dependencies, shared across requests and reconcile cycles.
	client, err := cloudrun.NewClient(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	st, err := g.openStore()
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	copier := registry.NewCopier()
	errOut := cmd.ErrOrStderr()

	// buildEngine reloads the manifest repo and assembles an Engine bound to
	// the shared Cloud Run client and store. Called per request and per
	// reconcile cycle so state stays in step with Git.
	buildEngine := func() (*core.Engine, error) {
		repo, err := g.loadRepo(errOut)
		if err != nil {
			return nil, err
		}
		return &core.Engine{
			Repo:     repo,
			CloudRun: cloudRunAdapter{client},
			Copier:   copier,
			Commit:   committerFunc(gitops.Commit),
			History:  st,
			Actor:    resolveActor(), // server identity until Phase 2 auth (#28)
		}, nil
	}

	srv := server.New(func(context.Context) (server.UseCases, func(), error) {
		e, err := buildEngine()
		if err != nil {
			return nil, func() {}, err
		}
		return e, func() {}, nil
	})

	path, handler := srv.Handler()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("/", web.Handler())

	var ctrl *controller.Controller
	if opts.reconcile {
		ctrl = controller.New(opts.interval, func(context.Context) (controller.Snapshot, func(), error) {
			e, err := buildEngine()
			if err != nil {
				return controller.Snapshot{}, func() {}, err
			}
			return controller.Snapshot{AutoEnvs: autoEnvs(e.Repo), Deployer: e}, func() {}, nil
		})
		ctrl.Logf = func(format string, args ...any) { fmt.Fprintf(errOut, format+"\n", args...) }
		mux.HandleFunc("POST /-/reconcile", func(w http.ResponseWriter, _ *http.Request) {
			ctrl.Trigger()
			w.WriteHeader(http.StatusAccepted)
		})
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	httpSrv := &http.Server{
		Addr:              opts.addr,
		Handler:           mux,
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Shut down gracefully when the command context is cancelled (SIGINT).
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	if ctrl != nil {
		go func() {
			if err := ctrl.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintln(errOut, "reconcile loop stopped:", err)
			}
		}()
		fmt.Fprintf(errOut, "reconcile loop enabled (interval %s)\n", opts.interval)
	}

	fmt.Fprintln(errOut, "wataridori serve listening on", opts.addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// autoEnvs returns the sorted names of policy:auto environments in repo.
func autoEnvs(repo *manifest.Repo) []string {
	var names []string
	for name, env := range repo.Config.Environments {
		if env.Policy == manifest.PolicyAuto {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
