package cmd

// seedFile is a file written by `contour init` to illustrate the store
// convention. Paths are slash-separated and relative to the store root.
type seedFile struct {
	rel  string
	body string
}

// seedFiles returns the sample files init writes into a new store. They double
// as documentation of the layout and as something for the read commands to
// show before the user adds their own content.
func seedFiles() []seedFile {
	return []seedFile{
		{"README.md", readmeSeed},
		{"bootstrap/go.md", bootstrapGoSeed},
		{"rules/general/010-communication.md", ruleCommunicationSeed},
		{"rules/go/010-errors.md", ruleErrorsSeed},
		{"skills/general/write-commit-message/SKILL.md", skillCommitSeed},
		{"skills/go/release/SKILL.md", skillReleaseSeed},
		{"knowledge/general/stack.md", knowledgeStackSeed},
	}
}

const readmeSeed = `# contour store

This directory is your contour store: the single source of truth for the
rules, skills and knowledge that contour feeds to your AI agents. Point the
CONTOUR_HOME environment variable at it to keep it anywhere on disk.

## Layout

    bootstrap/   Named entry points. Each profile selects, by tag, which
                 rules load eagerly and which skills/knowledge are exposed
                 on demand for a kind of project (e.g. a Go backend).
    rules/       Imperative "how to behave" guidance. Loaded eagerly.
    skills/      Procedural "how to do X". Fetched on demand.
    knowledge/   Reference facts. Fetched on demand.

## Conventions

- Folder names under rules/, skills/ and knowledge/ become implicit tags on
  the files beneath them. rules/go/errors.md is tagged "go".
- Files may start with optional YAML frontmatter for a description and extra
  tags:

      ---
      description: How we handle errors in Go
      tags: [errors, style]
      ---

- Ordering within a folder follows the filename. Prefix with a three-digit
  number (010-, 020-) when order matters. The gaps leave room to insert
  entries later, and three digits keep the ordering correct past 99 files
  in a folder, where 100- would otherwise sort before 20-.
- A skill is any directory that contains a SKILL.md file.

These sample files are just a starting point — edit or delete them freely.
`

const bootstrapGoSeed = `---
description: Go backend projects
rules: [general, go]
skills: [general, go]
knowledge: [general, go]
---

Entry point for Go backend projects. Rules tagged "general" and "go" are
loaded eagerly; skills and knowledge with those tags are offered on demand.
`

const ruleCommunicationSeed = `---
description: How the agent should communicate
---

- Be concise and direct; lead with the answer.
- Explain the reasoning behind non-obvious changes.
- Ask before doing anything irreversible.
`

const ruleErrorsSeed = `---
description: Error handling in Go
tags: [errors]
---

- Wrap errors with context using fmt.Errorf and %w.
- Return errors rather than panicking in library code.
- Keep error messages lower-case and free of trailing punctuation.
`

const skillCommitSeed = `---
description: Write a clear commit message
---

# Commit message

1. Summarise the change in one imperative line, under 72 characters.
2. Leave a blank line, then explain why the change was needed.
3. Reference the issue the change closes, if there is one.
`

const skillReleaseSeed = `---
description: Cut a tagged release of a Go module
---

# Release

1. Update the changelog.
2. Tag the commit with a semver tag (vX.Y.Z).
3. Push the tag to trigger the release workflow.
`

const knowledgeStackSeed = `---
description: The tools and libraries this developer prefers
---

Document your preferred stack here so agents pick the right tools without
being told each time.
`
