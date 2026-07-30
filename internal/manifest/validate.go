package manifest

import (
	"fmt"
	"regexp"
	"sort"
)

// validate checks wataridori.yaml rules from docs/spec/phase1-cli.md §1.1.
func (c *Config) validate() ([]string, error) {
	if c.Version != 1 {
		return nil, fmt.Errorf("unsupported version %d (only 1 is supported)", c.Version)
	}
	if len(c.Environments) == 0 {
		return nil, fmt.Errorf("no environments defined")
	}

	names := make([]string, 0, len(c.Environments))
	for name := range c.Environments {
		names = append(names, name)
	}
	sort.Strings(names)

	allAuto := true
	for _, name := range names {
		env := c.Environments[name]
		if err := env.validate(c); err != nil {
			return nil, fmt.Errorf("environment %q: %w", name, err)
		}
		if env.Policy != PolicyAuto {
			allAuto = false
		}
	}

	var warnings []string
	if allAuto {
		warnings = append(warnings, "all environments use policy \"auto\"; promotion never applies in this setup")
	}
	return warnings, nil
}

func (e *Environment) validate(c *Config) error {
	switch e.Policy {
	case PolicyAuto:
		if e.Branch == "" {
			return fmt.Errorf("policy \"auto\" requires \"branch\"")
		}
	case PolicyManual:
		if e.PromoteFrom != "" {
			if _, ok := c.Environments[e.PromoteFrom]; !ok {
				return fmt.Errorf("promoteFrom points to unknown environment %q", e.PromoteFrom)
			}
			if e.PromoteFrom == e.Name {
				return fmt.Errorf("promoteFrom must not point to itself")
			}
		}
	case "":
		return fmt.Errorf("\"policy\" is required (auto or manual)")
	default:
		return fmt.Errorf("unknown policy %q (must be auto or manual)", e.Policy)
	}
	if e.GCP.Project == "" || e.GCP.Region == "" {
		return fmt.Errorf("\"gcp.project\" and \"gcp.region\" are required")
	}
	if e.Services == "" {
		return fmt.Errorf("\"services\" (manifest directory) is required")
	}
	return nil
}

// serviceNameRE is Cloud Run's service-name rule: lowercase letters, digits
// and hyphens, starting with a letter and not ending in a hyphen.
var serviceNameRE = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

// validate checks service manifest rules from docs/spec/phase1-cli.md §1.2.
func (s *Service) validate() error {
	if s.Name == "" {
		return fmt.Errorf("\"name\" is required")
	}
	if s.Image == "" {
		return fmt.Errorf("\"image\" is required")
	}
	if _, _, err := SplitDigest(s.Image); err != nil {
		return err
	}
	// Only the name that reaches the API is format-checked; "name" is a
	// manifest-side identity and stays free-form when cloudRunName is set.
	if run := s.RunName(); !serviceNameRE.MatchString(run) || len(run) > 63 {
		return fmt.Errorf("%q is not a valid Cloud Run service name (lowercase letters, digits and hyphens, starting with a letter, at most 63 characters)", run)
	}
	if s.Scaling.Max != 0 && s.Scaling.Max < s.Scaling.Min {
		return fmt.Errorf("scaling.max (%d) must be >= scaling.min (%d)", s.Scaling.Max, s.Scaling.Min)
	}
	seen := make(map[string]bool, len(s.Env))
	for _, e := range s.Env {
		if err := e.validate(); err != nil {
			return err
		}
		if seen[e.Name] {
			return fmt.Errorf("env %q is declared twice", e.Name)
		}
		seen[e.Name] = true
	}
	return nil
}

func (e EnvVar) validate() error {
	if e.Name == "" {
		return fmt.Errorf("every env entry needs a \"name\"")
	}
	switch {
	case e.Value != "" && e.Secret != "":
		return fmt.Errorf("env %q sets both \"value\" and \"secret\"; use one", e.Name)
	case e.Version != "" && e.Secret == "":
		return fmt.Errorf("env %q sets \"version\" without \"secret\"", e.Name)
	}
	return nil
}
