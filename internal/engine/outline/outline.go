package outline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/iannil/jianwu/internal/archetypes"
	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/style"
)

// Generate produces an outline for the given input by calling the LLM.
// Stateless: takes input, returns outline. No workspace mutation.
func Generate(ctx context.Context, chatter llm.Chatter, in Input) (*book.Outline, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	data, err := buildPromptData(in)
	if err != nil {
		return nil, fmt.Errorf("build prompt data: %w", err)
	}

	sysPrompt, err := renderTemplate("system", mustLoadSystem(), data)
	if err != nil {
		return nil, err
	}
	userPrompt, err := renderTemplate("user", mustLoadUser(), data)
	if err != nil {
		return nil, err
	}

	schema, err := JSONSchema()
	if err != nil {
		return nil, fmt.Errorf("generate schema: %w", err)
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		JSONSchema: schema,
	}
	resp, err := chatter.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	var outline book.Outline
	if err := json.Unmarshal([]byte(resp.Content), &outline); err != nil {
		return nil, fmt.Errorf("parse outline JSON: %w (content was: %s)", err, truncate(resp.Content, 500))
	}
	return &outline, nil
}

func (in Input) validate() error {
	var missing []string
	if in.ArchetypeID == "" {
		missing = append(missing, "archetype_id")
	}
	if in.Topic == "" {
		missing = append(missing, "topic")
	}
	if in.Audience == "" {
		missing = append(missing, "audience")
	}
	if in.Depth == "" {
		missing = append(missing, "depth")
	}
	if in.Goal == "" {
		missing = append(missing, "goal")
	}
	if in.Length == "" {
		missing = append(missing, "length")
	}
	if in.Language == "" {
		missing = append(missing, "language")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

func buildPromptData(in Input) (promptData, error) {
	archs, err := archetypes.Load()
	if err != nil {
		return promptData{}, fmt.Errorf("load archetypes: %w", err)
	}
	arch, ok := archs[in.ArchetypeID]
	if !ok {
		return promptData{}, fmt.Errorf("archetype %q not found", in.ArchetypeID)
	}
	archYAML, err := yamlMarshalArchetype(arch)
	if err != nil {
		return promptData{}, err
	}

	samples, err := style.LoadSamples()
	if err != nil {
		return promptData{}, fmt.Errorf("load samples: %w", err)
	}
	sampleText, ok := samples[in.ArchetypeID]
	if !ok {
		sampleText = "(no samples for this archetype)"
	}

	books, err := corpus.Load()
	if err != nil {
		return promptData{}, fmt.Errorf("load corpus: %w", err)
	}
	var matches []*corpus.Book
	for _, b := range books {
		if b.Archetype == in.ArchetypeID {
			matches = append(matches, b)
		}
	}
	refs := referenceOutlines(matches)

	return promptData{
		Archetype:      archYAML,
		Samples:        sampleText,
		CorpusOutlines: refs,
		Topic:          in.Topic,
		Audience:       in.Audience,
		Depth:          in.Depth,
		Goal:           in.Goal,
		Length:         in.Length,
		Language:       in.Language,
	}, nil
}

func renderTemplate(name string, raw []byte, data any) (string, error) {
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute %s template: %w", name, err)
	}
	return buf.String(), nil
}

func mustLoadSystem() []byte {
	b, err := loadSystem()
	if err != nil {
		panic(err)
	}
	return b
}

func mustLoadUser() []byte {
	b, err := loadUser()
	if err != nil {
		panic(err)
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// yamlMarshalArchetype serializes an archetype back to YAML text for prompt injection.
// We don't have the original YAML bytes handy here (the loader parses them); re-serialize.
func yamlMarshalArchetype(a *archetypes.Archetype) (string, error) {
	// For S3 v1, use a minimal pretty-printed format instead of round-tripping YAML.
	// This is good enough — the LLM only needs the structure, not the original byte layout.
	var b strings.Builder
	b.WriteString("id: " + a.ID + "\n")
	b.WriteString("name_zh: " + a.Name.Zh + "\n")
	b.WriteString("name_en: " + a.Name.En + "\n")
	b.WriteString("description: " + a.Description + "\n")
	if len(a.WhenToUse.Goals) > 0 || len(a.WhenToUse.TopicTypes) > 0 || len(a.WhenToUse.AudienceFit) > 0 {
		b.WriteString("\nwhen_to_use:\n")
		if len(a.WhenToUse.Goals) > 0 {
			b.WriteString("  goals: [" + strings.Join(a.WhenToUse.Goals, ", ") + "]\n")
		}
		if len(a.WhenToUse.TopicTypes) > 0 {
			b.WriteString("  topic_types: [" + strings.Join(a.WhenToUse.TopicTypes, ", ") + "]\n")
		}
		if len(a.WhenToUse.AudienceFit) > 0 {
			b.WriteString("  audience_fit: [" + strings.Join(a.WhenToUse.AudienceFit, ", ") + "]\n")
		}
		if len(a.WhenToUse.NotRecommendedFor) > 0 {
			b.WriteString("  not_recommended_for: [" + strings.Join(a.WhenToUse.NotRecommendedFor, ", ") + "]\n")
		}
	}
	b.WriteString("\nparts:\n")
	for _, p := range a.Parts {
		b.WriteString("  - role: " + p.Role + "\n")
		b.WriteString("    title_template_zh: " + p.TitleTemplate.Zh + "\n")
		b.WriteString("    title_template_en: " + p.TitleTemplate.En + "\n")
		b.WriteString("    guidance: " + p.Guidance + "\n")
		b.WriteString("    typical_chapters: " + intsToStr(p.TypicalChapters) + "\n")
		b.WriteString("    chapter_role_hints:\n")
		for _, h := range p.ChapterRoleHints {
			b.WriteString("      - " + h + "\n")
		}
	}
	return b.String(), nil
}

func intsToStr(xs []int) string {
	var s []string
	for _, x := range xs {
		s = append(s, fmt.Sprintf("%d", x))
	}
	return "[" + strings.Join(s, ", ") + "]"
}
