package scaffolding

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// chapterSchema describes the fields the LLM must populate for one chapter.
// It's a subset of book.OutlineChapter focused on scaffolding fields.
type chapterSchema struct {
	Abstract           string   `json:"abstract" jsonschema:"description=该章在整本书中承担的角色和核心论点"`
	KeyConcepts        []string `json:"key_concepts" jsonschema:"description=3-7 个核心术语"`
	LearningObjectives []string `json:"learning_objectives" jsonschema:"description=2-4 条'读者能...'陈述"`
	SuggestedExamples  []string `json:"suggested_examples" jsonschema:"description=2-4 个例子/案例/思想实验"`
}

// JSONSchema returns the JSON Schema for chapter scaffolding output.
func JSONSchema() ([]byte, error) {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	s := r.Reflect(&chapterSchema{})
	return json.Marshal(s)
}
