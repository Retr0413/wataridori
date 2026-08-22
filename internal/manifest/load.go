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
	Root     string
	Config   *Config
	Warnings []string
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
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	return &Repo{Root: absRoot, Config: cfg, Warnings: warnings}, warnings, nil
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
	dir, err := r.servicesDir(env)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("environment %q: %w", env.Name, err)
	}
	var services []*Service
	seen := make(map[string]string)
	seenRunNames := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}
		if e.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("environment %q: service manifest %s must not be a symbolic link", env.Name, e.Name())
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
		if previous, ok := seen[svc.Name]; ok {
			return nil, fmt.Errorf("environment %q: service %q is declared in both %s and %s", env.Name, svc.Name, previous, rel)
		}
		seen[svc.Name] = rel
		if previous, ok := seenRunNames[svc.RunName()]; ok {
			return nil, fmt.Errorf("environment %q: Cloud Run service %q is targeted by both %s and %s", env.Name, svc.RunName(), previous, rel)
		}
		seenRunNames[svc.RunName()] = rel
		services = append(services, svc)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("environment %q: no service manifests found in %s", env.Name, env.Services)
	}
	return services, nil
}

func (r *Repo) servicesDir(env *Environment) (string, error) {
	dir := filepath.Join(r.Root, env.Services)
	resolvedRoot, err := filepath.EvalSymlinks(r.Root)
	if err != nil {
		return "", err
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("environment %q: %w", env.Name, err)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("environment %q: services directory %q escapes the repository root", env.Name, env.Services)
	}
	return dir, nil
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
