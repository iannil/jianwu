package expand

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// JSONSchemaResearch returns the schema for iter 1 output.
func JSONSchemaResearch() ([]byte, error) {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	s := r.Reflect(&ResearchNotes{})
	return json.Marshal(s)
}

// JSONSchemaValidation returns the schema for iter 3 output.
func JSONSchemaValidation() ([]byte, error) {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	s := r.Reflect(&ValidationResult{})
	return json.Marshal(s)
}
