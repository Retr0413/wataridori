package manifest

import (
	"path/filepath"
	"testing"
)

// TestExamplesAreValid keeps /examples loadable; the quickstart depends
// on them.
func TestExamplesAreValid(t *testing.T) {
	for _, dir := range []string{"simple", "split-registry"} {
		t.Run(dir, func(t *testing.T) {
			root := filepath.Join("..", "..", "examples", dir)
			repo, warnings, err := Load(root)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(warnings) != 0 {
				t.Errorf("warnings: %v", warnings)
			}
			for name, env := range repo.Config.Environments {
				if _, err := repo.LoadServices(env); err != nil {
					t.Errorf("environment %s: %v", name, err)
				}
			}
		})
	}
}
