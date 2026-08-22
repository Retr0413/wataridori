package workflow_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var reusableFiles = []string{
	"reusable-validate.yml",
	"reusable-dev-update-pr.yml",
	"reusable-dev-deploy.yml",
	"reusable-promotion-pr.yml",
	"reusable-prod-apply.yml",
}

func root(t *testing.T) string {
	t.Helper()
	r, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func read(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{root(t)}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func parse(t *testing.T, data []byte) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		t.Fatal("workflow must be a YAML mapping")
	}
	return doc.Content[0]
}

func mappingValue(t *testing.T, mapping *yaml.Node, key string) *yaml.Node {
	t.Helper()
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	t.Fatalf("missing key %q", key)
	return nil
}

func mappingKeys(t *testing.T, mapping *yaml.Node) []string {
	t.Helper()
	if mapping.Kind != yaml.MappingNode {
		t.Fatalf("want mapping node, got %v", mapping.Kind)
	}
	var keys []string
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
}

func TestReusableWorkflowsAreValidYAMLAndPinExternalActions(t *testing.T) {
	usesRE := regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*([^\s#]+)`)
	shaRE := regexp.MustCompile(`@[0-9a-f]{40}$`)
	for _, name := range reusableFiles {
		t.Run(name, func(t *testing.T) {
			data := read(t, ".github", "workflows", name)
			parse(t, data)
			for _, match := range usesRE.FindAllSubmatch(data, -1) {
				uses := string(match[1])
				if strings.HasPrefix(uses, "./") {
					continue
				}
				if !shaRE.MatchString(uses) {
					t.Errorf("external Action is not pinned by full SHA: %s", uses)
				}
			}
		})
	}
}

func TestProductionEntryPointsAreManualOnly(t *testing.T) {
	reusable := parse(t, read(t, ".github", "workflows", "reusable-prod-apply.yml"))
	if got := mappingKeys(t, mappingValue(t, reusable, "on")); strings.Join(got, ",") != "workflow_call" {
		t.Fatalf("reusable prod triggers = %v, want workflow_call only", got)
	}

	example := parse(t, read(t, "examples", "github-actions", "same-repository", "deploy-prod.yml"))
	if got := mappingKeys(t, mappingValue(t, example, "on")); strings.Join(got, ",") != "workflow_dispatch" {
		t.Fatalf("consumer prod triggers = %v, want workflow_dispatch only", got)
	}
}

func TestAutomationNeverExposesForceOrAutomaticProduction(t *testing.T) {
	var all bytes.Buffer
	for _, name := range reusableFiles {
		all.Write(read(t, ".github", "workflows", name))
	}
	text := all.String()
	if regexp.MustCompile(`(^|[[:space:]])--force([[:space:]]|$)`).MatchString(text) {
		t.Error("reusable workflows expose the Wataridori --force flag")
	}
	for _, forbidden := range []string{"gh pr merge", "enable-auto-merge"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("reusable workflows contain forbidden production automation %q", forbidden)
		}
	}

	promotion := string(read(t, ".github", "workflows", "reusable-promotion-pr.yml"))
	if strings.Contains(promotion, "wataridori apply") {
		t.Error("promotion PR workflow must not run apply")
	}
	prod := string(read(t, ".github", "workflows", "reusable-prod-apply.yml"))
	if !strings.Contains(prod, `GITHUB_EVENT_NAME" == "workflow_dispatch`) {
		t.Error("prod reusable workflow must reject non-manual callers")
	}
}

func TestSetupActionMetadataAndShell(t *testing.T) {
	metadata := parse(t, read(t, "actions", "setup", "action.yml"))
	inputs := mappingValue(t, metadata, "inputs")
	version := mappingValue(t, inputs, "version")
	required := mappingValue(t, version, "required")
	if required.Value != "true" {
		t.Error("setup Action version input must be required")
	}

	script := filepath.Join(root(t), "actions", "setup", "install.sh")
	cmd := exec.Command("bash", "-n", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s", err, out)
	}
}
