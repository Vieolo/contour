package store

import "strings"

// Origin records where an item came from.
type Origin string

const (
	// OriginStore is the central store (~/contour by default).
	OriginStore Origin = "store"
	// OriginLocal is a project overlay found in the working directory. Local
	// items apply unconditionally to their project and take precedence over
	// central ones on conflict.
	OriginLocal Origin = "local"
)

// Item is a single unit of content in the store: one rule, skill or knowledge
// entry. Items are produced by Load and served to agents through the CLI and
// the MCP server.
type Item struct {
	// Kind is the category the item belongs to.
	Kind Kind

	// Source records whether the item comes from the central store or a project
	// overlay.
	Source Origin

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

// LineMatch is a single body line that contains the search query.
type LineMatch struct {
	// Number is the 1-based line number within the item body, counted as the
	// body is returned by Get (i.e. after frontmatter has been stripped).
	Number int

	// Text is the full line. Presentation layers decide whether to truncate it.
	Text string

	// Count is the number of times the query occurs on this line.
	Count int
}

// Match describes how an item matched a search query: which parts of the item
// the query was found in and, for body matches, on which lines. It is the raw
// material both the CLI and the MCP search tool shape for their own output.
type Match struct {
	Item Item

	// InID, InDescription and InTags report matches in the item's metadata,
	// which carry no line locations.
	InID          bool
	InDescription bool
	InTags        bool

	// Lines are the body lines containing the query, in order.
	Lines []LineMatch

	// Occurrences is the total number of times the query appears in the body.
	Occurrences int
}

// MatchedIn lists the parts of the item the query was found in — any of "id",
// "description", "tags" and "body" — so a content match is distinguishable from
// a metadata-only one.
func (m Match) MatchedIn() []string {
	var where []string
	if m.InID {
		where = append(where, "id")
	}
	if m.InDescription {
		where = append(where, "description")
	}
	if m.InTags {
		where = append(where, "tags")
	}
	if len(m.Lines) > 0 {
		where = append(where, "body")
	}
	return where
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

// Matches reports whether the item satisfies a case-insensitive query across
// its ID, description, tags and body. An empty query matches every item.
func (it Item) Matches(query string) bool {
	_, ok := it.findMatch(query)
	return ok
}

// findMatch computes how the item matches a query, reporting false when it does
// not match at all. Matching is case-insensitive; an empty query matches every
// item with no recorded locations.
func (it Item) findMatch(query string) (Match, bool) {
	m := Match{Item: it}

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return m, true
	}

	m.InID = strings.Contains(strings.ToLower(it.ID), q)
	m.InDescription = strings.Contains(strings.ToLower(it.Description), q)
	for _, t := range it.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			m.InTags = true
			break
		}
	}

	for i, line := range strings.Split(it.Body, "\n") {
		if c := strings.Count(strings.ToLower(line), q); c > 0 {
			m.Lines = append(m.Lines, LineMatch{Number: i + 1, Text: line, Count: c})
			m.Occurrences += c
		}
	}

	matched := m.InID || m.InDescription || m.InTags || len(m.Lines) > 0
	return m, matched
}
