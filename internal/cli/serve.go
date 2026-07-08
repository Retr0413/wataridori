package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/gitops"
	"github.com/Retr0413/wataridori/internal/registry"
	"github.com/Retr0413/wataridori/internal/server"
)

func newServeCmd(g *globalFlags) *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Connect RPC server (Phase 2 controller/UI backend)",
		Long: "Serve the same use cases as the CLI over Connect RPC. The Cloud Run\n" +
			"client and history store are opened once; the manifest repo is reloaded\n" +
			"per request so it stays in step with Git.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, g, addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	return cmd
}

func runServe(cmd *cobra.Command, g *globalFlags, addr string) error {
	ctx := cmd.Context()

	// Long-lived dependencies, shared across requests.
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

	// engine reloads the manifest repo per request; Cloud Run and the store
	// are reused. Actor resolution stays server-side until Phase 2 auth (#28).
	engine := func(_ context.Context) (server.UseCases, func(), error) {
		repo, err := g.loadRepo(errOut)
		if err != nil {
			return nil, func() {}, err
		}
		e := &core.Engine{
			Repo:     repo,
			CloudRun: cloudRunAdapter{client},
			Copier:   copier,
			Commit:   committerFunc(gitops.Commit),
			History:  st,
			Actor:    resolveActor(),
		}
		return e, func() {}, nil
	}

	srv := server.New(engine)
	path, handler := srv.Handler()
	mux := http.NewServeMux()
	mux.Handle(path, handler)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}), // gRPC over cleartext
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Shut down gracefully when the command context is cancelled (SIGINT).
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	fmt.Fprintln(errOut, "wataridori serve listening on", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
