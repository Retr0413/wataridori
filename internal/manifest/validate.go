package manifest

import (
	"fmt"
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
	if s.Scaling.Max != 0 && s.Scaling.Max < s.Scaling.Min {
		return fmt.Errorf("scaling.max (%d) must be >= scaling.min (%d)", s.Scaling.Max, s.Scaling.Min)
	}
	return nil
}
