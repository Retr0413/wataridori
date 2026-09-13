package workflow_test

import (
	"os/exec"
	"testing"
)

// Snapshot builds skip real tag discovery, so validate the configured sort key
// with Git as well. GoReleaser OSS passes this value through to Git.
func TestReleaseTagSortIsSupportedByGit(t *testing.T) {
	config := parse(t, read(t, ".goreleaser.yaml"))
	sort := mappingValue(t, mappingValue(t, config, "git"), "tag_sort").Value
	cmd := exec.Command("git", "tag", "--list", "--sort="+sort)
	cmd.Dir = root(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("release tag_sort %q is incompatible with Git/GoReleaser OSS: %v\n%s", sort, err, output)
	}
}
