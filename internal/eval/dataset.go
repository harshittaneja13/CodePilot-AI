// Package eval is an offline evaluation harness for the review agent. It runs the
// agent (or any Reviewer) over a golden set of PRs with known issues and scores the
// findings by precision/recall/F1, so prompt/model changes can be measured and CI can
// fail on regressions.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ExpectedFinding describes an issue a good reviewer should surface for a case.
// Line is optional (0 = don't check). Keywords are matched case-insensitively against
// the produced finding's title+explanation; any one match counts.
type ExpectedFinding struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Keywords []string `json:"keywords"`
	Severity string   `json:"severity,omitempty"`
}

// CaseFile is one changed file in a case's PR. Content (full source) is optional and
// used only when a real agent fetches file contents via tools.
type CaseFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Patch    string `json:"patch"`
	Content  string `json:"content,omitempty"`
}

// CasePR is the pull request presented to the reviewer.
type CasePR struct {
	Title string     `json:"title"`
	Body  string     `json:"body"`
	Files []CaseFile `json:"files"`
}

// Case is one evaluation example: a PR plus the findings expected for it.
type Case struct {
	Name     string            `json:"name"`
	PR       CasePR            `json:"pr"`
	Expected []ExpectedFinding `json:"expected"`
}

// Load reads all *.json case files from dir (sorted by filename for stable ordering).
func Load(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading eval dir %q: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var cases []Case
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading case %q: %w", name, err)
		}
		var c Case
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("parsing case %q: %w", name, err)
		}
		if c.Name == "" {
			c.Name = name
		}
		cases = append(cases, c)
	}
	return cases, nil
}
