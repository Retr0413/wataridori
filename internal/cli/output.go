package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// printJSON renders a core result verbatim; this output doubles as the
// draft of the Phase 2 API responses.
func printJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// table writes aligned columns: header row then data rows.
func table(out io.Writer, header []string, rows [][]string) {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}

// shortImage abbreviates a digest-pinned reference for table cells,
// keeping the image name and a truncated digest.
func shortImage(image string) string {
	if image == "" {
		return "-"
	}
	path, digest, err := manifest.SplitDigest(image)
	if err != nil {
		return image
	}
	name := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		name = path[i+1:]
	}
	return name + "@" + manifest.ShortDigest(digest)
}

// confirm prints a y/N prompt and reads one line from in.
func confirm(out io.Writer, in io.Reader, question string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N]: ", question)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
