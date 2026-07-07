package main

import (
	"fmt"
	"os"
)

func main() {
	// CLI entrypoint is wired up in internal/cli (Phase 1, issue #7).
	fmt.Fprintln(os.Stderr, "wataridori: not implemented yet — see https://github.com/Retr0413/wataridori/milestone/1")
	os.Exit(1)
}
