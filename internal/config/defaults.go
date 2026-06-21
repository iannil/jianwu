package config

// BuiltinDefaults returns the lowest-precedence config layer.
// These values are used when neither global nor workspace config specifies
// a field.
func BuiltinDefaults() *Config {
	return &Config{
		SchemaVersion: 1,
		Models: Models{
			Intake:      ModelRef{Provider: "glm", Model: "glm-4.6"},
			Outline:     ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
			Scaffolding: ModelRef{Provider: "gemini", Model: "gemini-2.5-flash"},
			Expand:      ModelRef{Provider: "glm", Model: "glm-4.6"},
		},
		Search: Search{
			Primary: "brave", Fallback: "serper", Reader: "jina",
		},
		Archetypes: SourceOrder{Library: []string{"user", "builtin"}},
		Style: StyleSources{
			Guide:   []string{"user", "builtin"},
			Samples: []string{"builtin"},
		},
		Scaffolding: Scaffolding{Concurrency: 5},
		Logging:     Logging{Level: "warn"},
	}
}
