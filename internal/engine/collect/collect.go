// Package collect auto-builds workspace reference corpus for a topic:
// plan search queries → search the web → read pages → extract structured
// reference-book outlines with the LLM. There is no builtin corpus; collected
// books are the corpus consumed by outline generation and similar-book lookup.
//
// Run is pure with respect to the workspace: it returns validated books and
// the caller persists them (corpus.SaveBook) and rebuilds the index.
package collect

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/iannil/jianwu/internal/archetypes"
	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
	"github.com/iannil/jianwu/internal/llmjson"
)

// Limits bounding one collection run (cost control, mirrors expand's caps).
const (
	maxQueries       = 5
	maxSearchResults = 5
	maxCandidates    = 12
	maxPages         = 6
	maxPageRunes     = 6000
	defaultCount     = 3
	maxCount         = 8
	maxParts         = 6
	maxChapters      = 12
)

// Input describes one collection run.
type Input struct {
	// Topic is the subject to collect reference books for (required).
	Topic string
	// Audience is an optional grill audience hint ("scholar", …); "" lets the
	// LLM infer per book.
	Audience string
	// Count is the target number of books; 0 = default 3. Clamped to 1..8.
	Count int
	// Language of the collected corpus titles/abstracts; default "zh".
	Language string
}

// Deps bundles the providers the collector uses.
type Deps struct {
	Chatter  llm.Chatter
	Searcher search.Searcher
	Reader   reader.Reader
}

// Result summarizes one run. Books are validated and deduped; the caller
// decides which to persist.
type Result struct {
	Books  []*corpus.Book
	Issues []string // non-fatal problems (failed pages, dropped books…)
	Usage  engine.TokenUsage
}

// ProgressFunc receives (percent 0-100, message) updates.
type ProgressFunc func(percent int, msg string)

// Run executes one collection run.
func Run(ctx context.Context, deps Deps, in Input, progress ProgressFunc) (*Result, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Topic) == "" {
		return nil, fmt.Errorf("topic 为必填")
	}
	if in.Count <= 0 {
		in.Count = defaultCount
	}
	if in.Count > maxCount {
		in.Count = maxCount
	}
	if in.Language == "" {
		in.Language = "zh"
	}
	report := func(p int, msg string) {
		if progress != nil {
			progress(p, msg)
		}
	}

	tracker := &engine.TokenTracker{}
	res := &Result{}
	chatter := engine.NewTrackingChatter(deps.Chatter, tracker)

	// 1. Plan search queries (LLM, template fallback).
	report(5, "规划搜索词")
	queries, err := planQueries(ctx, chatter, in.Topic)
	if err != nil {
		res.Issues = append(res.Issues, fmt.Sprintf("搜索词规划降级为模板: %v", err))
		queries = templateQueries(in.Topic)
	}

	// 2. Search candidates.
	report(15, fmt.Sprintf("搜索 %d 个查询", len(queries)))
	seen := map[string]bool{}
	var urls []string
	for _, q := range queries {
		results, err := deps.Searcher.Search(ctx, q, search.SearchOpts{MaxResults: maxSearchResults})
		if err != nil {
			res.Issues = append(res.Issues, fmt.Sprintf("搜索 %q 失败: %v", q, err))
			continue
		}
		for _, r := range results {
			if r.URL == "" || seen[r.URL] {
				continue
			}
			seen[r.URL] = true
			urls = append(urls, r.URL)
			if len(urls) >= maxCandidates {
				break
			}
		}
		if len(urls) >= maxCandidates {
			break
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("搜索未返回任何候选页面")
	}

	// 3. Read pages.
	report(25, fmt.Sprintf("阅读 %d 个页面", min(len(urls), maxPages)))
	var pages []string
	for i, u := range urls {
		if len(pages) >= maxPages {
			break
		}
		report(25+i*8, fmt.Sprintf("阅读 %s", u))
		content, err := deps.Reader.Read(ctx, u)
		if err != nil {
			res.Issues = append(res.Issues, fmt.Sprintf("阅读 %s 失败: %v", u, err))
			continue
		}
		text := truncateRunes(content.Markdown, maxPageRunes)
		if strings.TrimSpace(text) == "" {
			continue
		}
		pages = append(pages, fmt.Sprintf("[%d] 来源: %s\n标题: %s\n\n%s", len(pages)+1, u, content.Title, text))
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("所有候选页面均无法读取")
	}

	// 4. Extract structured books with the LLM.
	report(70, "LLM 提取结构化语料")
	archIDs, err := archetypeIDs()
	if err != nil {
		return nil, err
	}
	raw, err := extractBooks(ctx, chatter, in, archIDs, pages)
	if err != nil {
		return nil, err
	}

	// 5. Normalize, validate, dedupe.
	report(90, "校验与归一")
	archSet := map[string]bool{}
	for _, id := range archIDs {
		archSet[id] = true
	}
	kept := map[string]bool{}
	for _, b := range raw.Books {
		normalized, ok := normalizeBook(b, archSet, in, urls)
		if !ok {
			res.Issues = append(res.Issues, fmt.Sprintf("丢弃无效条目: %q（缺标题/原型/结构）", b.TitleZh))
			continue
		}
		if kept[normalized.Slug] {
			res.Issues = append(res.Issues, fmt.Sprintf("重复 slug %q，仅保留第一个", normalized.Slug))
			continue
		}
		kept[normalized.Slug] = true
		res.Books = append(res.Books, normalized)
		if len(res.Books) >= in.Count {
			break
		}
	}
	if len(res.Books) == 0 {
		return nil, fmt.Errorf("未能从页面中提取到任何结构化语料（可更换主题或稍后重试）")
	}

	report(100, fmt.Sprintf("采集完成：%d 本", len(res.Books)))
	res.Usage = tracker.Snapshot()
	return res, nil
}

func (d Deps) validate() error {
	var missing []string
	if d.Chatter == nil {
		missing = append(missing, "chatter")
	}
	if d.Searcher == nil {
		missing = append(missing, "searcher")
	}
	if d.Reader == nil {
		missing = append(missing, "reader")
	}
	if len(missing) > 0 {
		return fmt.Errorf("collect 缺少依赖: %s", strings.Join(missing, ", "))
	}
	return nil
}

// planQueries asks the LLM for diverse search queries; one per line.
func planQueries(ctx context.Context, chatter llm.Chatter, topic string) ([]string, error) {
	resp, err := chatter.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "你是检索规划助手。为主题生成 5 个多样化的中文/英文网络搜索查询，用于发现与主题相关的高质量图书、书评与长文。每行一个查询，不要编号，不要解释。"},
			{Role: "user", Content: "主题: " + topic},
		},
	})
	if err != nil {
		return nil, err
	}
	var queries []string
	for _, line := range strings.Split(resp.Content, "\n") {
		q := strings.TrimSpace(line)
		q = strings.Trim(q, "-*0123456789. ")
		if q != "" {
			queries = append(queries, q)
		}
		if len(queries) >= maxQueries {
			break
		}
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("空回复")
	}
	return queries, nil
}

// templateQueries is the deterministic fallback query plan.
func templateQueries(topic string) []string {
	return []string{
		topic + " 书籍 推荐",
		topic + " 书 目录",
		"best books about " + topic,
		topic + " book review table of contents",
		topic + " 图书 豆瓣",
	}
}

// extractedBook/part/chapter are the JSON shapes the LLM must return.
type extractedBook struct {
	Slug       string          `json:"slug"`
	TitleZh    string          `json:"title_zh"`
	TitleEn    string          `json:"title_en"`
	Archetype  string          `json:"archetype"`
	Audience   string          `json:"audience"`
	Depth      string          `json:"depth"`
	Goal       string          `json:"goal"`
	Length     string          `json:"length"`
	Abstract   string          `json:"abstract"`
	SourceName string          `json:"source_name"`
	SourceURL  string          `json:"source_url"`
	Parts      []extractedPart `json:"parts"`
}

type extractedPart struct {
	TitleZh  string             `json:"title_zh"`
	Role     string             `json:"role"`
	Chapters []extractedChapter `json:"chapters"`
}

type extractedChapter struct {
	TitleZh  string `json:"title_zh"`
	Abstract string `json:"abstract"`
}

// extractionView is the top-level extraction response.
type extractionView struct {
	Books []extractedBook `json:"books"`
}

func extractionSchema() string {
	return `{"type":"object","required":["books"],"properties":{"books":{"type":"array","items":{"type":"object","required":["title_zh","archetype","parts"],"properties":{"slug":{"type":"string"},"title_zh":{"type":"string"},"title_en":{"type":"string"},"archetype":{"type":"string"},"audience":{"type":"string"},"depth":{"type":"string"},"goal":{"type":"string"},"length":{"type":"string"},"abstract":{"type":"string"},"source_name":{"type":"string"},"source_url":{"type":"string"},"parts":{"type":"array","items":{"type":"object","required":["title_zh","chapters"],"properties":{"title_zh":{"type":"string"},"role":{"type":"string"},"chapters":{"type":"array","items":{"type":"object","required":["title_zh"],"properties":{"title_zh":{"type":"string"},"abstract":{"type":"string"}}}}}}}}}}}}`
}

// extractBooks makes the extraction LLM call with page contents.
func extractBooks(ctx context.Context, chatter llm.Chatter, in Input, archIDs, pages []string) (*extractionView, error) {
	audienceHint := in.Audience
	if audienceHint == "" {
		audienceHint = "(由内容推断)"
	}
	sys := strings.Join([]string{
		"你是图书语料采集助手。从给定的网页内容中提取与主题相关的、真实存在的图书或结构化长文的框架信息。",
		"输出 JSON：{\"books\": [...]}。",
		"规则:",
		"- 只提取页面内容能支撑的条目（目录、书评、章节结构、长文章节结构均可）；宁缺毋滥。",
		"- slug 用小写英文与短横线，如 time-reality；无法确定时根据英文书名生成。",
		"- archetype 必须从这个列表中选择: " + strings.Join(archIDs, ", ") + "。",
		"- audience 取值: scholar, advanced-practitioner, educated-general, beginner。",
		"- depth 取值: intro, intermediate, advanced。goal 取值: understanding, operational, decision。",
		"- length 取值: short, medium, long。",
		"- parts 1-6 个，每个 part 含 1-12 个 chapters；title_zh 必填；role 从 ontology/mesontology/practice/epistemology/methodology/history/synthesis 中选择或留空。",
		"- source_url 填该条目主要依据的网页 URL；source_name 填页面标题或书名。",
		"- 条目之间不要重复；最多输出 " + fmt.Sprint(maxCount) + " 本。",
	}, "\n")
	user := strings.Join([]string{
		"主题: " + in.Topic,
		"受众提示: " + audienceHint,
		"语料语言: " + in.Language,
		"",
		"网页内容:",
		strings.Join(pages, "\n\n---\n\n"),
	}, "\n")

	resp, err := chatter.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		JSONSchema: []byte(extractionSchema()),
	})
	if err != nil {
		return nil, fmt.Errorf("提取调用失败: %w", err)
	}
	var view extractionView
	if err := llmjson.Unmarshal(resp.Content, &view); err != nil {
		return nil, fmt.Errorf("解析提取结果: %w (content: %s)", err, truncateRunes(resp.Content, 300))
	}
	return &view, nil
}

// archetypeIDs returns the valid archetype IDs (sorted for stable prompts).
func archetypeIDs() ([]string, error) {
	archs, err := archetypes.Load()
	if err != nil {
		return nil, fmt.Errorf("load archetypes: %w", err)
	}
	ids := make([]string, 0, len(archs))
	for id := range archs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

var slugPattern = regexp.MustCompile(`[^a-z0-9-]+`)

// validGrillValues are the accepted grill parameter values (mirrors grill tree).
var (
	validAudiences = map[string]bool{"scholar": true, "advanced-practitioner": true, "educated-general": true, "beginner": true}
	validDepths    = map[string]bool{"intro": true, "intermediate": true, "advanced": true}
	validGoals     = map[string]bool{"understanding": true, "operational": true, "decision": true}
	validLengths   = map[string]bool{"short": true, "medium": true, "long": true}
)

// normalizeBook validates and coerces one extraction entry into a corpus.Book;
// ok=false means the entry is unusable.
func normalizeBook(b extractedBook, archSet map[string]bool, in Input, candidateURLs []string) (*corpus.Book, bool) {
	if strings.TrimSpace(b.TitleZh) == "" || !archSet[b.Archetype] {
		return nil, false
	}
	slug := normalizeSlug(b.Slug, b.TitleEn, b.TitleZh)
	if slug == "" {
		return nil, false
	}
	audience := b.Audience
	if !validAudiences[audience] {
		audience = "educated-general"
	}
	depth := b.Depth
	if !validDepths[depth] {
		depth = "intermediate"
	}
	goal := b.Goal
	if !validGoals[goal] {
		goal = "understanding"
	}
	length := b.Length
	if !validLengths[length] {
		length = "medium"
	}

	out := &corpus.Book{
		Slug:      slug,
		Title:     corpus.LocalizedTitle{Zh: strings.TrimSpace(b.TitleZh), En: strings.TrimSpace(b.TitleEn)},
		Archetype: b.Archetype,
		Audience:  audience,
		Depth:     depth,
		Goal:      goal,
		Length:    length,
		Language:  []string{in.Language},
		Abstract:  strings.TrimSpace(b.Abstract),
		Source: corpus.Source{
			Name:       firstNonEmpty(b.SourceName, "web"),
			URL:        firstNonEmpty(b.SourceURL, firstNonEmpty(candidateURLs...)),
			AccessedAt: time.Now().UTC().Format("2006-01-02"),
		},
	}
	for _, p := range b.Parts {
		if len(out.Parts) >= maxParts {
			break
		}
		if strings.TrimSpace(p.TitleZh) == "" || len(p.Chapters) == 0 {
			continue
		}
		cp := corpus.Part{
			Index: len(out.Parts) + 1,
			Title: corpus.LocalizedTitle{Zh: strings.TrimSpace(p.TitleZh)},
			Role:  strings.TrimSpace(p.Role),
		}
		for _, c := range p.Chapters {
			if len(cp.Chapters) >= maxChapters {
				break
			}
			if strings.TrimSpace(c.TitleZh) == "" {
				continue
			}
			cp.Chapters = append(cp.Chapters, corpus.Chapter{
				Index:    len(cp.Chapters) + 1,
				Title:    corpus.LocalizedTitle{Zh: strings.TrimSpace(c.TitleZh)},
				Abstract: strings.TrimSpace(c.Abstract),
			})
		}
		if len(cp.Chapters) > 0 {
			out.Parts = append(out.Parts, cp)
		}
	}
	if len(out.Parts) == 0 {
		return nil, false
	}
	return out, true
}

// normalizeSlug coerces the LLM-provided slug into [a-z0-9-]; falls back to a
// slugified English title, then pinyin-less fallback of the zh title is not
// attempted — an empty result means "unusable".
func normalizeSlug(raw, titleEn, titleZh string) string {
	candidates := []string{raw, titleEn, titleZh}
	for _, c := range candidates {
		s := strings.ToLower(strings.TrimSpace(c))
		s = strings.ReplaceAll(s, " ", "-")
		s = slugPattern.ReplaceAllString(s, "-")
		s = strings.Trim(s, "-")
		s = regexp.MustCompile(`-{2,}`).ReplaceAllString(s, "-")
		if s != "" {
			return s
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
