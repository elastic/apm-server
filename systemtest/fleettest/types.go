package fleettest

import "time"

// Agent holds details of a Fleet Agent.
type Agent struct {
	ID                   string                 `json:"id"`
	Active               bool                   `json:"active"`
	Status               string                 `json:"status"`
	Type                 string                 `json:"type"`
	PolicyID             string                 `json:"policy_id,omitempty"`
	EnrolledAt           time.Time              `json:"enrolled_at,omitempty"`
	UserProvidedMetadata map[string]interface{} `json:"user_provided_metadata,omitempty"`
	LocalMetadata        map[string]interface{} `json:"local_metadata,omitempty"`
}

// AgentPolicy holds details of a Fleet Agent Policy.
type AgentPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	Revision    int    `json:"revision"`

	Agents            int       `json:"agents"`
	IsDefault         bool      `json:"is_default"`
	MonitoringEnabled []string  `json:"monitoring_enabled"`
	PackagePolicies   []string  `json:"package_policies"`
	Status            string    `json:"status"`
	UpdatedAt         time.Time `json:"updated_at"`
	UpdatedBy         string    `json:"updated_by"`
}

// PackagePolicy holds details of a Fleet Package Policy.
type PackagePolicy struct {
	ID            string               `json:"id,omitempty"`
	Name          string               `json:"name"`
	Namespace     string               `json:"namespace"`
	Enabled       bool                 `json:"enabled"`
	Description   string               `json:"description"`
	AgentPolicyID string               `json:"policy_id"`
	OutputID      string               `json:"output_id"`
	Inputs        []PackagePolicyInput `json:"inputs"`
	Package       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Title   string `json:"title"`
	} `json:"package"`
}

type PackagePolicyInput struct {
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Streams []interface{}          `json:"streams"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Vars    map[string]interface{} `json:"vars,omitempty"`
}

type Package struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Release     string `json:"release"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Download    string `json:"download"`
	Path        string `json:"path"`
	Status      string `json:"status"`
}
