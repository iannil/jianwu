package grill

// Dimension is one node in the design tree.
type Dimension struct {
	ID           string   // e.g. "topic", "audience"
	Name         string   // human-readable, e.g. "核心问题"
	Description  string   // one-line description of what this dimension captures
	Question     string   // the question asked to the user
	Options      []string // suggested options (LLM may add more in recommendations)
	DependsOn    []string // dimension IDs that must be answered first
	Trigger      Trigger  // when to ask (Always, or Conditional on a predicate)
	Required     bool     // true = must answer; false = can skip with default
	DefaultValue string   // used if user skips
}

// Trigger decides whether a conditional dimension should be asked.
type Trigger struct {
	Kind      TriggerKind
	Dimension string // for Kind=Equals: which dimension's answer to check
	Value     string // for Kind=Equals: what value triggers asking
}

type TriggerKind int

const (
	TriggerAlways TriggerKind = iota // ask unconditionally
	TriggerEquals                    // ask only if Dimension's answer == Value
)

// Core dimensions per DESIGN.md §5.2 (6 dimensions).
// Conditional dimensions are 6 more, gated by Trigger.
func DefaultTree() *DesignTree {
	return &DesignTree{
		Dimensions: []Dimension{
			{
				ID:           "topic",
				Name:         "核心问题",
				Description:  "这本书回答什么问题",
				Question:     "你想写一本关于什么主题的书？这本书要回答的核心问题是什么？",
				Options:      []string{},
				DependsOn:    []string{},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     true,
				DefaultValue: "",
			},
			{
				ID:           "audience",
				Name:         "受众",
				Description:  "为谁而写",
				Question:     "这本书的目标读者是谁？",
				Options:      []string{"scholar", "advanced-practitioner", "educated-general", "beginner"},
				DependsOn:    []string{"topic"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     true,
				DefaultValue: "educated-general",
			},
			{
				ID:           "goal",
				Name:         "目标",
				Description:  "理解/实操/决策",
				Question:     "读者读完这本书应该获得什么？",
				Options:      []string{"understanding", "operational", "decision"},
				DependsOn:    []string{"audience"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     true,
				DefaultValue: "understanding",
			},
			{
				ID:          "archetype",
				Name:        "结构原型",
				Description: "书的骨架来自哪个原型",
				Question:    "这本书用哪种结构组织？",
				Options: []string{
					"ontology-epistemology-practice",
					"diagnosis-decoding-breakthrough",
					"foundations-application-practice",
				},
				DependsOn:    []string{"goal"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     true,
				DefaultValue: "ontology-epistemology-practice",
			},
			{
				ID:           "depth",
				Name:         "深度",
				Description:  "入门/进阶/专家",
				Question:     "内容的深度？",
				Options:      []string{"intro", "intermediate", "advanced"},
				DependsOn:    []string{"audience"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     true,
				DefaultValue: "intermediate",
			},
			{
				ID:           "length",
				Name:         "篇幅",
				Description:  "短/中/长",
				Question:     "书的篇幅？",
				Options:      []string{"short", "medium", "long"},
				DependsOn:    []string{"archetype", "depth"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     true,
				DefaultValue: "medium",
			},
			// --- Conditional dimensions (6) ---
			{
				ID:           "language",
				Name:         "语言",
				Description:  "zh / en / bilingual",
				Question:     "书的语言？",
				Options:      []string{"zh", "en", "bilingual"},
				DependsOn:    []string{"topic"},
				Trigger:      Trigger{Kind: TriggerAlways}, // we ask by default since jianwu is bilingual
				Required:     true,
				DefaultValue: "zh",
			},
			{
				ID:           "scope",
				Name:         "范围",
				Description:  "单本/卷/章",
				Question:     "范围（如果主题很大，可以聚焦到卷或章）？",
				Options:      []string{"single", "volume", "chapter"},
				DependsOn:    []string{"topic"},
				Trigger:      Trigger{Kind: TriggerAlways}, // ask unconditionally; user can skip
				Required:     false,
				DefaultValue: "single",
			},
			{
				ID:           "example_type",
				Name:         "例子类型",
				Description:  "案例/思想实验/数据",
				Question:     "偏好哪种类型的例子？",
				Options:      []string{"case", "thought_experiment", "data", "mixed"},
				DependsOn:    []string{"topic"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     false,
				DefaultValue: "mixed",
			},
			{
				ID:          "citation_style",
				Name:        "引用风格",
				Description: "学术/通俗/无",
				Question:    "引用风格？",
				Options:     []string{"academic", "popular", "none"},
				DependsOn:   []string{"audience"},
				Trigger: Trigger{
					Kind:      TriggerEquals,
					Dimension: "audience",
					Value:     "scholar",
				},
				Required:     false,
				DefaultValue: "popular",
			},
			{
				ID:           "visualization",
				Name:         "可视化",
				Description:  "图表/无",
				Question:     "是否需要图表？",
				Options:      []string{"charts", "tables", "none"},
				DependsOn:    []string{"topic"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     false,
				DefaultValue: "tables",
			},
			{
				ID:           "timeliness",
				Name:         "时效",
				Description:  "永恒/当下/前瞻",
				Question:     "内容的时效取向？",
				Options:      []string{"timeless", "current", "forward"},
				DependsOn:    []string{"topic"},
				Trigger:      Trigger{Kind: TriggerAlways},
				Required:     false,
				DefaultValue: "timeless",
			},
		},
	}
}

// DesignTree is the full tree of dimensions.
type DesignTree struct {
	Dimensions []Dimension
}

// Find returns the dimension with the given ID, or nil.
func (t *DesignTree) Find(id string) *Dimension {
	for i := range t.Dimensions {
		if t.Dimensions[i].ID == id {
			return &t.Dimensions[i]
		}
	}
	return nil
}

// Walk returns dimensions in dependency order, filtered by which should be
// asked given the answers so far. Conditional dimensions whose Trigger isn't
// met are skipped (their DefaultValue is applied implicitly).
func (t *DesignTree) Walk(answers map[string]string) []Dimension {
	var ordered []Dimension
	added := map[string]bool{}

	// Simple iterative topological-ish walk: repeat until no more can be added.
	// The tree is small (12 dimensions); this O(n²) is fine.
	for {
		progress := false
		for _, d := range t.Dimensions {
			if added[d.ID] {
				continue
			}
			// All dependencies must be explicitly answered.
			depsMet := true
			for _, dep := range d.DependsOn {
				if _, ok := answers[dep]; !ok {
					depsMet = false
					break
				}
			}
			if !depsMet {
				continue
			}
			// Trigger check.
			if !t.triggerMet(d, answers) {
				added[d.ID] = true // mark as processed (skipped)
				progress = true
				continue
			}
			ordered = append(ordered, d)
			added[d.ID] = true
			progress = true
		}
		if !progress {
			break
		}
	}
	return ordered
}

func (t *DesignTree) triggerMet(d Dimension, answers map[string]string) bool {
	if d.Trigger.Kind == TriggerAlways {
		return true
	}
	if d.Trigger.Kind == TriggerEquals {
		return answers[d.Trigger.Dimension] == d.Trigger.Value
	}
	return false
}
