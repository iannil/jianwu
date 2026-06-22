package outline

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/zhurong/jianwu/internal/book"
)

// JSONSchema generates a JSON Schema describing the expected outline structure.
// Used as response_format in ChatRequest to enforce structured LLM output.
func JSONSchema() ([]byte, error) {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	s := r.Reflect(&book.Outline{})
	return json.Marshal(s)
}
