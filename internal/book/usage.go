package book

// TokenUsage accumulates reported provider tokens; missing reports are not estimates.
// Old books start with no recorded calls; historical consumption is unknown.
type TokenUsage struct {
	PromptTokens      int `json:"prompt_tokens"`
	CompletionTokens  int `json:"completion_tokens"`
	TotalTokens       int `json:"total_tokens"`
	CallCount         int `json:"call_count"`
	CachedCount       int `json:"cached_count"`
	MissingUsageCalls int `json:"missing_usage_calls"`
}

// Add merges another run into the cumulative usage.
func (u *TokenUsage) Add(v TokenUsage) {
	u.PromptTokens += v.PromptTokens
	u.CompletionTokens += v.CompletionTokens
	u.TotalTokens += v.TotalTokens
	u.CallCount += v.CallCount
	u.CachedCount += v.CachedCount
	u.MissingUsageCalls += v.MissingUsageCalls
}
