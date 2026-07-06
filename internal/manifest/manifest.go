package manifest

// Policy controls how an environment gets updated.
type Policy string

const (
	// PolicyAuto follows a branch automatically (reconciled in Phase 2).
	PolicyAuto Policy = "auto"
	// PolicyManual only updates through an explicit promote.
	PolicyManual Policy = "manual"
)

// ConfigFileName is the environment definition file at the repository root.
const ConfigFileName = "wataridori.yaml"

// Config is the parsed wataridori.yaml.
type Config struct {
	Version      int                     `yaml:"version"`
	Environments map[string]*Environment `yaml:"environments"`
}

// Environment is one deploy target (dev, staging, prod, ...).
type Environment struct {
	// Name is the map key in wataridori.yaml, filled in by the loader.
	Name        string     `yaml:"-"`
	Policy      Policy     `yaml:"policy"`
	Branch      string     `yaml:"branch,omitempty"`
	PromoteFrom string     `yaml:"promoteFrom,omitempty"`
	GCP         GCP        `yaml:"gcp"`
	Services    string     `yaml:"services"`
	ImageCopy   *ImageCopy `yaml:"imageCopy,omitempty"`
}

// GCP locates the Cloud Run project/region of an environment.
type GCP struct {
	Project string `yaml:"project"`
	Region  string `yaml:"region"`
}

// ImageCopy configures cross-registry image copy on promote, for setups
// where each environment has its own Artifact Registry repository.
type ImageCopy struct {
	To string `yaml:"to"`
}

// Service is one per-environment service manifest (one file = one service).
type Service struct {
	Name           string    `yaml:"name"`
	Image          string    `yaml:"image"`
	Env            []EnvVar  `yaml:"env,omitempty"`
	Resources      Resources `yaml:"resources,omitempty"`
	Scaling        Scaling   `yaml:"scaling,omitempty"`
	ServiceAccount string    `yaml:"serviceAccount,omitempty"`
	Concurrency    int32     `yaml:"concurrency,omitempty"`
	Port           int32     `yaml:"port,omitempty"`

	// File is the manifest path relative to the repository root,
	// filled in by the loader.
	File string `yaml:"-"`
}

// EnvVar is a literal environment variable.
type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Resources are per-instance limits.
type Resources struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// Scaling bounds instance counts.
type Scaling struct {
	Min int32 `yaml:"min"`
	Max int32 `yaml:"max,omitempty"`
}
