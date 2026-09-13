package cloudrun

import (
	"context"
	"fmt"
	"slices"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	runpb "cloud.google.com/go/run/apiv2/runpb"
	"github.com/Retr0413/wataridori/internal/manifest"
	iam "google.golang.org/api/iam/v1"
)

// DeployPermissions probes effective permissions without changing IAM or Cloud
// Run. API failures are unknown, never proof that deployment is authorized.
func (c *Client) DeployPermissions(ctx context.Context, env *manifest.Environment, name string) (map[string]bool, error) {
	resource := ServiceName(env, name)
	required := []string{"run.services.get", "run.services.update", "run.revisions.get"}
	permissions, err := c.services.TestIamPermissions(ctx, &iampb.TestIamPermissionsRequest{Resource: resource, Permissions: required})
	if err != nil {
		return nil, fmt.Errorf("cloud run permission probe: %w", err)
	}
	result := map[string]bool{}
	for _, permission := range required {
		result[permission] = slices.Contains(permissions.Permissions, permission)
	}
	svc, err := c.services.GetService(ctx, &runpb.GetServiceRequest{Name: resource})
	if err != nil {
		return result, fmt.Errorf("reading deploy target: %w", err)
	}
	account := svc.GetTemplate().GetServiceAccount()
	if account == "" {
		return result, fmt.Errorf("runtime service account is unknown; cannot verify iam.serviceAccounts.actAs")
	}
	client, err := iam.NewService(ctx)
	if err != nil {
		return result, err
	}
	response, err := client.Projects.ServiceAccounts.TestIamPermissions("projects/-/serviceAccounts/"+account, &iam.TestIamPermissionsRequest{Permissions: []string{"iam.serviceAccounts.actAs"}}).Context(ctx).Do()
	if err != nil {
		return result, fmt.Errorf("runtime service account permission probe: %w", err)
	}
	result["iam.serviceAccounts.actAs"] = slices.Contains(response.Permissions, "iam.serviceAccounts.actAs")
	return result, nil
}
