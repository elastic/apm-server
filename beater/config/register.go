package config

// RegisterConfig holds ingest config information
type RegisterConfig struct {
	Ingest *IngestConfig `config:"ingest"`
}

// IngestConfig holds config pipeline ingest information
type IngestConfig struct {
	Pipeline *PipelineConfig `config:"pipeline"`
}

// PipelineConfig holds config information about registering ingest pipelines
type PipelineConfig struct {
	Enabled   *bool `config:"enabled"`
	Overwrite *bool `config:"overwrite"`
	Path      string
}
