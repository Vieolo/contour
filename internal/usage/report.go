package usage

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vieolo/contour/internal/config"
)

// Tally is a counted key — a search query or an item ID — with the time it was
// last seen.
type Tally struct {
	Key      string
	Count    int
	LastSeen time.Time
}

// Report is the aggregate of the usage logs.
type Report struct {
	Sessions int
	Projects int
	Since    time.Time // zero means all time

	// Gaps are searches that returned nothing — the most actionable signal,
	// keyed by query.
	Gaps []Tally

	// Fetches are items successfully fetched, keyed by item ID.
	Fetches []Tally

	// MissingFetches are gets for IDs that did not exist — broken references,
	// keyed by the requested ID.
	MissingFetches []Tally
}

// Options filters an aggregation.
type Options struct {
	// ProjectSubstr keeps only events whose project path contains this string,
	// case-insensitively. Empty means every project.
	ProjectSubstr string

	// Since keeps only events at or after this time. Zero means all time.
	Since time.Time
}

// Aggregate reads every session log and summarises it. A missing usage
// directory is not an error — it just means nothing has been recorded yet.
func Aggregate(opts Options) (*Report, error) {
	dir, err := config.UsageDir()
	if err != nil {
		return nil, err
	}

	var (
		gaps     = map[string]*Tally{}
		fetches  = map[string]*Tally{}
		missing  = map[string]*Tally{}
		projects = map[string]bool{}
		sessions int
		substr   = strings.ToLower(strings.TrimSpace(opts.ProjectSubstr))
	)

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			var e Event
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue // skip a malformed line rather than fail the whole read
			}
			if substr != "" && !strings.Contains(strings.ToLower(e.Project), substr) {
				continue
			}
			ts, _ := time.Parse(time.RFC3339, e.Time)
			if !opts.Since.IsZero() && ts.Before(opts.Since) {
				continue
			}

			projects[e.Project] = true
			switch e.Type {
			case "session_start":
				sessions++
			case "search":
				if e.Results != nil && *e.Results == 0 {
					bump(gaps, e.Query, ts)
				}
			case "get":
				if e.Found != nil && *e.Found {
					bump(fetches, e.ID, ts)
				} else {
					bump(missing, e.ID, ts)
				}
			}
		}
		return sc.Err()
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return &Report{
		Sessions:       sessions,
		Projects:       len(projects),
		Since:          opts.Since,
		Gaps:           ranked(gaps),
		Fetches:        ranked(fetches),
		MissingFetches: ranked(missing),
	}, nil
}

// FetchedKeys returns the set of item IDs that were fetched, so a caller can
// work out which store items were never touched.
func (r *Report) FetchedKeys() map[string]bool {
	set := make(map[string]bool, len(r.Fetches))
	for _, t := range r.Fetches {
		set[t.Key] = true
	}
	return set
}

func bump(m map[string]*Tally, key string, ts time.Time) {
	if key == "" {
		return
	}
	t := m[key]
	if t == nil {
		t = &Tally{Key: key}
		m[key] = t
	}
	t.Count++
	if ts.After(t.LastSeen) {
		t.LastSeen = ts
	}
}

// ranked orders tallies by count (desc), then most-recent, then key, so the
// output is deterministic and leads with what matters most.
func ranked(m map[string]*Tally) []Tally {
	out := make([]Tally, 0, len(m))
	for _, t := range m {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		switch {
		case out[i].Count != out[j].Count:
			return out[i].Count > out[j].Count
		case !out[i].LastSeen.Equal(out[j].LastSeen):
			return out[i].LastSeen.After(out[j].LastSeen)
		default:
			return out[i].Key < out[j].Key
		}
	})
	return out
}
