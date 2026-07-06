package cli

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
)

// resolveActor identifies who ran the command for the history log:
// the ADC service-account email when available, otherwise user@host.
// User-credential ADC files carry no email, so this is best effort
// (spec §3).
func resolveActor() string {
	if email := adcClientEmail(); email != "" {
		return email
	}
	name := "unknown"
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	host, err := os.Hostname()
	if err != nil {
		return name
	}
	return name + "@" + host
}

func adcClientEmail() string {
	path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var creds struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return creds.ClientEmail
}
