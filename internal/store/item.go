package store

// Item is a single unit of content in the store: one rule, skill or knowledge
// entry. Items are produced by Load and served to agents through the CLI and
// the MCP server.
type Item struct {
	// Kind is the category the item belongs to.
	Kind Kind

	// ID is the item's stable identifier: its path relative to the store root,
	// slash-separated and without the file extension (e.g. "rules/python/errors").
	// For skills it is the skill directory's path (e.g. "skills/python/release").
	ID string

	// Name is the last segment of the ID (e.g. "errors", "release").
	Name string

	// Path is the absolute path to the item's file on disk. For skills this is
	// the SKILL.md inside the skill directory.
	Path string

	// Description is the item's one-line summary from frontmatter. It may be
	// empty and is what lets an agent judge relevance before fetching the body.
	Description string

	// Tags are the implicit tags (each folder segment under the kind root)
	// unioned with any explicit tags from frontmatter, de-duplicated.
	Tags []string

	// Body is the item's content with the frontmatter removed and surrounding
	// whitespace trimmed.
	Body string
}

// HasTag reports whether the item carries the given tag.
func (it Item) HasTag(tag string) bool {
	for _, t := range it.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
