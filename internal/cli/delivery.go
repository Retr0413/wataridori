package cli

import (
	"fmt"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/spf13/cobra"
)

func newDeliveryCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "delivery", Short: "Read delivery snapshots and verify an exact deployed artifact"}
	var env, service string
	snapshot := &cobra.Command{Use: "snapshot", Short: "Read one service's delivery metadata without credentials", RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := g.loadRepo(cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		environment, err := r.Environment(env)
		if err != nil {
			return err
		}
		services, err := r.LoadServices(environment)
		if err != nil {
			return err
		}
		for _, svc := range services {
			if svc.Name == service {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"environment": env, "service": service, "file": svc.File, "image": svc.Image,
					"runName": svc.RunName(), "applyMode": svc.EffectiveApplyMode(), "policy": environment.Policy,
					"promoteFrom": environment.PromoteFrom, "project": environment.GCP.Project, "region": environment.GCP.Region,
				})
			}
		}
		return fmt.Errorf("service %q is not in environment %q", service, env)
	}}
	snapshot.Flags().StringVar(&env, "env", "", "environment")
	snapshot.Flags().StringVar(&service, "service", "", "one logical service")
	_ = snapshot.MarkFlagRequired("env")
	_ = snapshot.MarkFlagRequired("service")
	var verifyEnv, verifyService, image string
	var paths []string
	verify := &cobra.Command{Use: "verify", Short: "Verify desired/actual digest, Ready, traffic, registry and HTTP", RunE: func(cmd *cobra.Command, _ []string) error {
		e, cleanup, err := g.engine(cmd, engineOptions{needRepo: true, needCloudRun: true, needVerifier: true, needHTTP: true})
		defer cleanup()
		if err != nil {
			return err
		}
		r, verifyErr := e.VerifyDeployment(cmd.Context(), verifyEnv, verifyService, image, paths)
		if r != nil {
			if verifyErr != nil {
				r.Reasons = append(r.Reasons, verifyErr.Error())
			}
			if err := printJSON(cmd.OutOrStdout(), r); err != nil {
				return err
			}
		}
		if verifyErr != nil {
			return verifyErr
		}
		if !r.Verified {
			return fmt.Errorf("deployment verification failed")
		}
		return nil
	}}
	verify.Flags().StringVar(&verifyEnv, "env", "", "environment")
	verify.Flags().StringVar(&verifyService, "service", "", "one logical service")
	verify.Flags().StringVar(&image, "image", "", "exact selected immutable image")
	verify.Flags().StringSliceVar(&paths, "http-path", nil, "relative health paths (HTTP 200 required)")
	for _, f := range []string{"env", "service", "image"} {
		_ = verify.MarkFlagRequired(f)
	}
	var preflightEnv, preflightService string
	preflight := &cobra.Command{Use: "preflight", Short: "Read effective Cloud Run update and runtime service-account permissions", RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := g.loadRepo(cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		env, err := r.Environment(preflightEnv)
		if err != nil {
			return err
		}
		services, err := r.LoadServices(env)
		if err != nil {
			return err
		}
		for _, svc := range services {
			if svc.Name == preflightService {
				client, err := cloudrun.NewClient(cmd.Context())
				if err != nil {
					return err
				}
				defer func() { _ = client.Close() }()
				permissions, probeErr := client.DeployPermissions(cmd.Context(), env, svc.RunName())
				state := "passed"
				if probeErr != nil {
					state = "unknown"
				} else {
					for _, allowed := range permissions {
						if !allowed {
							state = "blocked"
						}
					}
				}
				result := map[string]any{"status": state, "environment": preflightEnv, "service": preflightService, "permissions": permissions}
				if probeErr != nil {
					result["reason"] = probeErr.Error()
				}
				if err := printJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
				if state != "passed" {
					return fmt.Errorf("deploy permission preflight %s; verify WIF identity and least-privilege IAM", state)
				}
				return nil
			}
		}
		return fmt.Errorf("unknown service %q", preflightService)
	}}
	preflight.Flags().StringVar(&preflightEnv, "env", "", "environment")
	preflight.Flags().StringVar(&preflightService, "service", "", "logical service")
	_ = preflight.MarkFlagRequired("env")
	_ = preflight.MarkFlagRequired("service")
	cmd.AddCommand(snapshot, verify, preflight)
	return cmd
}
