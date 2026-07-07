package metrics

// Default is the process-wide registry the application records into.
var Default = NewRegistry()

var (
	reviewsTotal = Default.NewCounter(
		"codepilot_reviews_total", "Reviews processed, by final status.", "status")

	reviewTokens = Default.NewCounter(
		"codepilot_review_tokens_total", "LLM tokens consumed by reviews, by direction.", "direction")

	reviewCost = Default.NewCounter(
		"codepilot_review_cost_usd_total", "Estimated LLM cost of reviews, in USD.")

	agentToolCalls = Default.NewCounter(
		"codepilot_agent_tool_calls_total", "Tool calls made by the review agent.")

	reviewDuration = Default.NewHistogram(
		"codepilot_review_duration_seconds", "End-to-end review pipeline duration.",
		[]float64{1, 5, 10, 30, 60, 120, 300})

	httpRequests = Default.NewCounter(
		"codepilot_http_requests_total", "HTTP requests, by method/path/status.", "method", "path", "status")

	httpDuration = Default.NewHistogram(
		"codepilot_http_request_duration_seconds", "HTTP request duration.",
		[]float64{0.005, 0.025, 0.1, 0.5, 1, 2, 5}, "method", "path")
)

// ReviewCompleted records a finished review by status ("completed" or "failed").
func ReviewCompleted(status string) { reviewsTotal.Inc(status) }

// AddReviewTokens records input/output tokens for a review.
func AddReviewTokens(input, output int) {
	reviewTokens.Add(float64(input), "input")
	reviewTokens.Add(float64(output), "output")
}

// AddReviewCost records the USD cost of a review.
func AddReviewCost(usd float64) { reviewCost.Add(usd) }

// AddAgentToolCalls records how many tools the agent invoked.
func AddAgentToolCalls(n int) {
	if n > 0 {
		agentToolCalls.Add(float64(n))
	}
}

// ObserveReviewDuration records the review pipeline duration in seconds.
func ObserveReviewDuration(seconds float64) { reviewDuration.Observe(seconds) }

// ObserveHTTP records an HTTP request's outcome and latency.
func ObserveHTTP(method, path, status string, seconds float64) {
	httpRequests.Inc(method, path, status)
	httpDuration.Observe(seconds, method, path)
}

// Render exposes the default registry in Prometheus text format.
func Render() string { return Default.Render() }
