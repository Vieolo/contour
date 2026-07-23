package store

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// frontmatter is the metadata block parsed from the top of an item file.
// Unknown fields are ignored, so authors can add their own without breaking
// loading.
type frontmatter struct {
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

// SplitFrontmatter separates an optional leading YAML frontmatter block —
// delimited by lines containing only "---" — from the body that follows. When
// no valid (opened and closed) frontmatter is present, the whole content is
// returned as the body and ok is false.
//
// It is exported so other packages — bootstrap profiles, for instance — can
// reuse the same file convention with their own metadata schema.
func SplitFrontmatter(content []byte) (fm string, body string, ok bool) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t\r") != FrontmatterDelim {
		return "", string(content), false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t\r") == FrontmatterDelim {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	// Opened but never closed: not valid frontmatter.
	return "", string(content), false
}

// parseItemFile splits an item file into its metadata and trimmed body.
func parseItemFile(content []byte) (frontmatter, string, error) {
	fmText, body, ok := SplitFrontmatter(content)

	var fm frontmatter
	if ok {
		if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
			return frontmatter{}, "", err
		}
	}
	return fm, strings.TrimSpace(body), nil
}
