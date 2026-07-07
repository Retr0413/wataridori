package manifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Repo is a loaded manifest repository.
type Repo struct {
	// Root is the absolute path of the directory containing wataridori.yaml.
	Root   string
	Config *Config
}

// FindRoot walks upwards from dir until it finds wataridori.yaml.
func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ConfigFileName)); err == nil {
			return d, nil
		}
		if d == filepath.Dir(d) {
			return "", fmt.Errorf("%s not found in %s or any parent directory", ConfigFileName, abs)
		}
	}
}

// Load reads and validates wataridori.yaml at root. Non-fatal findings are
// returned as warnings.
func Load(root string) (*Repo, []string, error) {
	data, err := os.ReadFile(filepath.Join(root, ConfigFileName))
	if err != nil {
		return nil, nil, err
	}
	cfg := &Config{}
	if err := decodeStrict(data, cfg); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ConfigFileName, err)
	}
	for name, env := range cfg.Environments {
		if env == nil {
			return nil, nil, fmt.Errorf("%s: environment %q is empty", ConfigFileName, name)
		}
		env.Name = name
	}
	warnings, err := cfg.validate()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ConfigFileName, err)
	}
	return &Repo{Root: root, Config: cfg}, warnings, nil
}

// Environment returns the named environment or an error listing the
// available ones.
func (r *Repo) Environment(name string) (*Environment, error) {
	if env, ok := r.Config.Environments[name]; ok {
		return env, nil
	}
	names := make([]string, 0, len(r.Config.Environments))
	for n := range r.Config.Environments {
		names = append(names, n)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("unknown environment %q (available: %s)", name, strings.Join(names, ", "))
}

// LoadServices reads every service manifest (*.yaml, *.yml) in the
// environment's services directory, sorted by file name.
func (r *Repo) LoadServices(env *Environment) ([]*Service, error) {
	dir := filepath.Join(r.Root, env.Services)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("environment %q: %w", env.Name, err)
	}
	var services []*Service
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}
		rel := filepath.Join(env.Services, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		svc := &Service{}
		if err := decodeStrict(data, svc); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		svc.File = rel
		if err := svc.validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		services = append(services, svc)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("environment %q: no service manifests found in %s", env.Name, env.Services)
	}
	return services, nil
}

// UpdateServiceImage rewrites the image reference of svc's manifest file by
// textual replacement, preserving YAML formatting and comments. The updated
// value is also set on svc.
func (r *Repo) UpdateServiceImage(svc *Service, newImage string) error {
	path := filepath.Join(r.Root, svc.File)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	old := []byte(svc.Image)
	if n := bytes.Count(data, old); n != 1 {
		return fmt.Errorf("%s: expected exactly one occurrence of %q, found %d", svc.File, svc.Image, n)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, bytes.Replace(data, old, []byte(newImage), 1), info.Mode()); err != nil {
		return err
	}
	svc.Image = newImage
	return nil
}

func decodeStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}

func isYAML(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}
