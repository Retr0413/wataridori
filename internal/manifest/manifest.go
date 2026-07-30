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
	// Name is the service's identity across environments: promotion matches
	// source to target by it, and the UI groups a service's environments into
	// one row by it. It defaults to being the Cloud Run service name too.
	Name string `yaml:"name"`
	// CloudRunName overrides the Cloud Run service name for this environment.
	// Deployments that encode the environment in the service name
	// (my-api-dev / my-api-prod, often because both live in one project)
	// would otherwise have no name in common to promote along.
	CloudRunName   string    `yaml:"cloudRunName,omitempty"`
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

// RunName is the Cloud Run service name to read and write: CloudRunName when
// set, otherwise Name. Every Cloud Run API call goes through this; Name alone
// is the manifest-side identity.
func (s *Service) RunName() string {
	if s.CloudRunName != "" {
		return s.CloudRunName
	}
	return s.Name
}

// EnvVar is one environment variable: either a literal Value or a reference
// to a Secret Manager secret, never both.
type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value,omitempty"`
	// Secret is a Secret Manager secret name — "my-secret" in the same
	// project, or "projects/{project}/secrets/{name}" elsewhere. Apply
	// replaces the service wholesale, so a secret-backed variable that the
	// manifest cannot express would be dropped from the running service.
	Secret string `yaml:"secret,omitempty"`
	// Version is the secret version; empty means SecretVersionLatest.
	Version string `yaml:"version,omitempty"`
}

// SecretVersionLatest is the default secret version — the same default the
// Cloud Run console applies.
const SecretVersionLatest = "latest"

// SecretVersion returns the version to bind, defaulting to latest.
func (e EnvVar) SecretVersion() string {
	if e.Version != "" {
		return e.Version
	}
	return SecretVersionLatest
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
