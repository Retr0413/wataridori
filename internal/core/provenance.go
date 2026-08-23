package core

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	repositoryRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	commitRE     = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

// Provenance identifies the application CI execution that produced an image.
type Provenance struct {
	SourceRepository string `json:"sourceRepository"`
	SourceCommit     string `json:"sourceCommit"`
	WorkflowRun      string `json:"workflowRun"`
}

func (p Provenance) validate() error {
	if !repositoryRE.MatchString(p.SourceRepository) {
		return fmt.Errorf("source repository must be owner/repository")
	}
	if !commitRE.MatchString(p.SourceCommit) {
		return fmt.Errorf("source commit must be a full 40-character hexadecimal SHA")
	}
	u, err := url.Parse(p.WorkflowRun)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("workflow run must be a github.com HTTPS URL without credentials, query, or fragment")
	}
	if !strings.Contains(u.Path, "/actions/runs/") {
		return fmt.Errorf("workflow run URL must identify a GitHub Actions run")
	}
	return nil
}
