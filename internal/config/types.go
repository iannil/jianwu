package config

// Config is the fully-resolved configuration for a workspace.
// Layers (low to high precedence): built-in defaults < global user config
// < workspace config < env vars < CLI flags.
// Env var and CLI flag overrides are applied by the CLI layer; Load returns
// the merged result of the three file-backed layers.
type Config struct {
	SchemaVersion int          `yaml:"schema_version"`
	LLM           LLMConfig    `yaml:"llm"`
	Models        Models       `yaml:"models"`
	Search        Search       `yaml:"search"`
	Archetypes    SourceOrder  `yaml:"archetypes"`
	Style         StyleSources `yaml:"style"`
	Scaffolding   Scaffolding  `yaml:"scaffolding"`
	Logging       Logging      `yaml:"logging"`
}

// LLMConfig holds global LLM settings shared across stages.
type LLMConfig struct {
	TimeoutSeconds int `yaml:"timeout"` // global default per-chat timeout (seconds); 0 = no timeout
}

type Models struct {
	Intake      ModelRef `yaml:"intake"`
	Outline     ModelRef `yaml:"outline"`
	Scaffolding ModelRef `yaml:"scaffolding"`
	Expand      ModelRef `yaml:"expand"`
}

// ModelRef names a provider+model for a stage.
// Fallback is optional; nil means no fallback configured.
// TimeoutSeconds overrides the global LLM timeout for this stage; 0 = use global default.
type ModelRef struct {
	Provider       string    `yaml:"provider"`
	Model          string    `yaml:"model"`
	Fallback       *ModelRef `yaml:"fallback,omitempty"`
	TimeoutSeconds int       `yaml:"timeout,omitempty"`
}

type Search struct {
	Primary  string `yaml:"primary"`
	Fallback string `yaml:"fallback"`
	Reader   string `yaml:"reader"`
}

type SourceOrder struct {
	Library []string `yaml:"library"`
}

type StyleSources struct {
	Guide   []string `yaml:"guide"`
	Samples []string `yaml:"samples"`
}

type Scaffolding struct {
	Concurrency int `yaml:"concurrency"`
}

type Logging struct {
	Level string `yaml:"level"`
}
