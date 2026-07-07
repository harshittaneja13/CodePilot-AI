package analyzer

import (
	"regexp"
)

// Pattern defines a regex-based code pattern to match against source lines.
type Pattern struct {
	Name     string         `json:"name"`
	Regex    string         `json:"regex"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Compiled *regexp.Regexp `json:"-"`
}

// compilePatterns compiles regex strings in a slice of Pattern definitions.
func compilePatterns(patterns []Pattern) []Pattern {
	compiled := make([]Pattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			// Skip patterns that fail to compile. In production code this
			// would be caught by unit tests.
			continue
		}
		p.Compiled = re
		compiled = append(compiled, p)
	}
	return compiled
}

// SecurityPatterns returns regex patterns for detecting security issues.
func SecurityPatterns() []Pattern {
	return compilePatterns([]Pattern{
		{
			Name:     "hardcoded-secret",
			Regex:    `(?i)(api[_-]?key|api[_-]?secret|access[_-]?token|auth[_-]?token|secret[_-]?key)\s*[:=]\s*["'][\w/+=]{8,}["']`,
			Severity: "critical",
			Message:  "Possible hardcoded secret or API key detected",
		},
		{
			Name:     "hardcoded-password",
			Regex:    `(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']{4,}["']`,
			Severity: "critical",
			Message:  "Possible hardcoded password detected",
		},
		{
			Name:     "private-key",
			Regex:    `(?i)-----BEGIN\s+(RSA|DSA|EC|OPENSSH)\s+PRIVATE\s+KEY-----`,
			Severity: "critical",
			Message:  "Private key embedded in source code",
		},
		{
			Name:     "sql-injection",
			Regex:    `(?i)(fmt\.Sprintf|string\.Format|f["'])\s*\(\s*["'].*?(SELECT|INSERT|UPDATE|DELETE|DROP)\s+.*?%[sv]`,
			Severity: "critical",
			Message:  "Potential SQL injection: user input interpolated into SQL query",
		},
		{
			Name:     "sql-concat",
			Regex:    `(?i)(SELECT|INSERT|UPDATE|DELETE)\s+.*?\+\s*(req\.|request\.|params\.|input\.|user)`,
			Severity: "critical",
			Message:  "Potential SQL injection: query built via string concatenation with user input",
		},
		{
			Name:     "xss-innerhtml",
			Regex:    `(?i)(innerHTML|outerHTML|document\.write)\s*[=(]`,
			Severity: "high",
			Message:  "Potential XSS vulnerability: direct DOM manipulation with innerHTML/outerHTML",
		},
		{
			Name:     "xss-dangerously-set",
			Regex:    `dangerouslySetInnerHTML`,
			Severity: "high",
			Message:  "Potential XSS vulnerability: dangerouslySetInnerHTML used",
		},
		{
			Name:     "insecure-hash",
			Regex:    `(?i)(md5|sha1)\s*[\.(]`,
			Severity: "high",
			Message:  "Insecure hash algorithm (MD5/SHA1); use SHA-256 or better",
		},
		{
			Name:     "insecure-random",
			Regex:    `(?i)(math\.rand|math/rand|random\.random\(\)|Random\(\))`,
			Severity: "medium",
			Message:  "Insecure random number generator; use crypto/rand for security-sensitive operations",
		},
		{
			Name:     "tls-skip-verify",
			Regex:    `InsecureSkipVerify\s*:\s*true`,
			Severity: "high",
			Message:  "TLS certificate verification disabled (InsecureSkipVerify)",
		},
		{
			Name:     "exec-injection",
			Regex:    `(?i)(exec\.Command|os\.system|subprocess\.(call|run|Popen)|child_process\.exec)\s*\(`,
			Severity: "high",
			Message:  "Command execution detected; ensure inputs are sanitized to prevent injection",
		},
		{
			Name:     "cors-wildcard",
			Regex:    `(?i)(Access-Control-Allow-Origin|allowOrigin|cors)\s*[:=]\s*["']\*["']`,
			Severity: "medium",
			Message:  "CORS wildcard origin detected; restrict to specific domains",
		},
	})
}

// QualityPatterns returns regex patterns for detecting code quality issues.
func QualityPatterns() []Pattern {
	return compilePatterns([]Pattern{
		{
			Name:     "race-condition-go",
			Regex:    `go\s+func\s*\(`,
			Severity: "medium",
			Message:  "Goroutine launched with closure; ensure shared variables are not accessed unsafely",
		},
		{
			Name:     "empty-catch",
			Regex:    `(?i)(catch|except)\s*(\([^)]*\))?\s*\{\s*\}`,
			Severity: "medium",
			Message:  "Empty catch/except block swallows errors silently",
		},
		{
			Name:     "console-log",
			Regex:    `console\.(log|debug|info|warn|error)\s*\(`,
			Severity: "low",
			Message:  "Console logging statement found; remove or use a proper logger",
		},
		{
			Name:     "print-debug",
			Regex:    `(?:^|\s)(fmt\.Print(ln|f)?|print\(|println\()`,
			Severity: "low",
			Message:  "Debug print statement found; use structured logging instead",
		},
		{
			Name:     "panic-in-library",
			Regex:    `\bpanic\s*\(`,
			Severity: "medium",
			Message:  "panic() used; prefer returning errors in library code",
		},
		{
			Name:     "global-variable",
			Regex:    `^var\s+\w+\s+`,
			Severity: "low",
			Message:  "Package-level variable detected; consider dependency injection",
		},
		{
			Name:     "deep-import",
			Regex:    `(?i)import\s+.*internal/`,
			Severity: "low",
			Message:  "Importing from internal package of another module may cause coupling",
		},
		{
			Name:     "http-status-number",
			Regex:    `(?:WriteHeader|StatusCode\s*==?|status\s*[:=])\s*\d{3}`,
			Severity: "low",
			Message:  "Prefer named HTTP status constants over numeric literals",
		},
	})
}

// magicNumberPatterns detects suspicious numeric literals in code.
var magicNumberPatterns = compilePatterns([]Pattern{
	{
		Name:     "magic-number",
		Regex:    `(?:==|!=|>=|<=|>|<|=)\s*\d{3,}`,
		Severity: "low",
		Message:  "Magic number in comparison/assignment",
	},
	{
		Name:     "magic-number-mult",
		Regex:    `\*\s*\d{3,}`,
		Severity: "low",
		Message:  "Magic number in multiplication",
	},
})
