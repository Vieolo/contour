// Package store models the contour store: the on-disk tree of rules, skills and
// knowledge that contour serves to AI agents.
//
// This file defines the layout vocabulary — the directory names and file
// conventions — shared by the loader and the `init` scaffolder, so the on-disk
// contract has a single source of truth.
package store

import (
	"fmt"
	"strings"
)

// Kind is a category of item in the store. Each Kind is also the name of a
// top-level directory under the store root.
type Kind string

const (
	KindRules     Kind = "rules"
	KindSkills    Kind = "skills"
	KindKnowledge Kind = "knowledge"
)

// Kinds lists every item Kind in the fixed order contour presents them.
var Kinds = []Kind{KindRules, KindSkills, KindKnowledge}

const (
	// BootstrapDir is the store subdirectory holding bootstrap profiles. It is
	// not a Kind: profiles compose the other kinds rather than being served as
	// content themselves.
	BootstrapDir = "bootstrap"

	// SkillFile is the filename that marks a directory as a skill. The
	// directory's name is the skill's name; the folders above it are tags.
	SkillFile = "SKILL.md"

	// MarkdownExt is the extension of item and profile files.
	MarkdownExt = ".md"

	// FrontmatterDelim delimits the optional YAML frontmatter block at the top
	// of an item or profile file.
	FrontmatterDelim = "---"
)

// ParseKind maps a user-supplied kind name to a Kind, accepting the singular
// form too. It backs both the CLI's kind argument and the MCP tools' kind
// parameter so the two accept exactly the same values.
func ParseKind(name string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "rules", "rule":
		return KindRules, nil
	case "skills", "skill":
		return KindSkills, nil
	case "knowledge":
		return KindKnowledge, nil
	default:
		return "", fmt.Errorf("unknown kind %q (want: rules, skills or knowledge)", name)
	}
}
