package scaffold

// seedFile is a sample file written into a new store to illustrate the
// convention. Paths are slash-separated and relative to the store root.
type seedFile struct {
	rel  string
	body string
}

// seedFiles returns the sample files written into a new store. They double as
// documentation of the layout and as something for the read commands to show
// before the user adds their own content.
func seedFiles() []seedFile {
	return []seedFile{
		{"README.md", readmeSeed},
		{"bootstrap/python.md", bootstrapPythonSeed},
		{"rules/general/010-communication.md", ruleCommunicationSeed},
		{"rules/python/010-errors.md", ruleErrorsSeed},
		{"skills/general/write-commit-message/SKILL.md", skillCommitSeed},
		{"skills/python/release/SKILL.md", skillReleaseSeed},
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
                 on demand for a kind of project (e.g. a Python backend).
    rules/       Imperative "how to behave" guidance. Loaded eagerly.
    skills/      Procedural "how to do X". Fetched on demand.
    knowledge/   Reference facts. Fetched on demand.

## Conventions

- Folder names under rules/, skills/ and knowledge/ become implicit tags on
  the files beneath them. rules/python/errors.md is tagged "python".
- Files may start with optional YAML frontmatter for a description and extra
  tags:

      ---
      description: How we handle errors in Python
      tags: [errors, style]
      ---

- Ordering within a folder follows the filename. Prefix with a three-digit
  number (010-, 020-) when order matters. The gaps leave room to insert
  entries later, and three digits keep the ordering correct past 99 files
  in a folder, where 100- would otherwise sort before 20-.
- A skill is any directory that contains a SKILL.md file.

These sample files are just a starting point — edit or delete them freely.
`

const bootstrapPythonSeed = `---
description: Python backend projects
rules: [general, python]
skills: [general, python]
knowledge: [general, python]
---

Entry point for Python backend projects. Rules tagged "general" and "python"
are loaded eagerly; skills and knowledge with those tags are offered on demand.
`

const ruleCommunicationSeed = `---
description: How the agent should communicate
---

- Be concise and direct; lead with the answer.
- Explain the reasoning behind non-obvious changes.
- Ask before doing anything irreversible.
`

const ruleErrorsSeed = `---
description: Error handling in Python
tags: [errors]
---

- Raise a specific exception type; never raise or catch a bare Exception.
- Chain with "raise WrappedError(...) from err" so the original traceback survives.
- Keep try blocks narrow: wrap only the call that can actually fail.
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
description: Cut a tagged release of a Python package
---

# Release

1. Update the changelog.
2. Bump the version in pyproject.toml.
3. Tag the commit with a semver tag (vX.Y.Z).
4. Push the tag to trigger the release workflow.
`

const knowledgeStackSeed = `---
description: The tools and libraries this developer prefers
---

Document your preferred stack here so agents pick the right tools without
being told each time.
`
