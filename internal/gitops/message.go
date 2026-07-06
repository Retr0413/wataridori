package gitops

import "fmt"

// PromoteMessage builds the conventional promote commit message, e.g.
// "promote(prod): my-app to abc123def456 (from dev)".
func PromoteMessage(toEnv, service, shortDigest, fromEnv string) string {
	return fmt.Sprintf("promote(%s): %s to %s (from %s)", toEnv, service, shortDigest, fromEnv)
}
