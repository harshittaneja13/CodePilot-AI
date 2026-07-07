package llm

import "strings"

// Usage tracks token counts split by direction so cost can be computed at the
// per-model prices (input and output tokens are priced differently).
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Add accumulates token counts.
func (u *Usage) Add(inputTokens, outputTokens int) {
	u.InputTokens += inputTokens
	u.OutputTokens += outputTokens
}

// Total returns the combined token count.
func (u Usage) Total() int { return u.InputTokens + u.OutputTokens }

// ModelPrice is the list price in USD per 1,000,000 tokens.
type ModelPrice struct {
	InputPerM  float64
	OutputPerM float64
}

type modelPriceEntry struct {
	match string // matched as a lowercase substring of the model id
	price ModelPrice
}

// priceTable maps models to approximate list prices (USD per 1M tokens). Entries are
// checked in order and the first substring match wins, so more specific ids come first.
// Groq free-tier models (llama/mixtral/gemma) are $0. Unknown models cost $0 (see CostUSD).
var priceTable = []modelPriceEntry{
	// OpenAI
	{"gpt-4o-mini", ModelPrice{0.15, 0.60}},
	{"gpt-4o", ModelPrice{2.50, 10.00}},
	{"gpt-4-turbo", ModelPrice{10.00, 30.00}},
	{"gpt-4", ModelPrice{30.00, 60.00}},
	{"gpt-3.5", ModelPrice{0.50, 1.50}},
	// Anthropic
	{"claude-3-5-haiku", ModelPrice{0.80, 4.00}},
	{"claude-3-haiku", ModelPrice{0.25, 1.25}},
	{"claude-3-5-sonnet", ModelPrice{3.00, 15.00}},
	{"claude-3-7-sonnet", ModelPrice{3.00, 15.00}},
	{"claude-sonnet", ModelPrice{3.00, 15.00}},
	{"claude-3-opus", ModelPrice{15.00, 75.00}},
	{"claude-opus", ModelPrice{15.00, 75.00}},
	// Groq free tier (and common OSS models served free)
	{"llama", ModelPrice{0, 0}},
	{"mixtral", ModelPrice{0, 0}},
	{"gemma", ModelPrice{0, 0}},
	{"qwen", ModelPrice{0, 0}},
	{"deepseek", ModelPrice{0, 0}},
}

// CostUSD returns the dollar cost of a single call given its token split.
// Unknown models return 0 (cost tracking degrades gracefully rather than guessing).
func CostUSD(model string, inputTokens, outputTokens int) float64 {
	price, ok := priceFor(model)
	if !ok {
		return 0
	}
	return float64(inputTokens)/1e6*price.InputPerM + float64(outputTokens)/1e6*price.OutputPerM
}

func priceFor(model string) (ModelPrice, bool) {
	m := strings.ToLower(model)
	for _, e := range priceTable {
		if strings.Contains(m, e.match) {
			return e.price, true
		}
	}
	return ModelPrice{}, false
}
