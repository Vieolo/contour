package bootstrap

import "github.com/vieolo/contour/internal/store"

// CrossRef is the cross-reference between a store's items and the profiles that
// select them: for each item, which entry points offer it at the start of a
// session.
//
// An item no profile selects is not out of reach. The list, search and get tools
// serve the whole store regardless of profile, so an agent can always find it.
// What it loses is disclosure: nothing puts it in front of the agent up front,
// so it is only ever used if the agent goes looking for it. That difference —
// offered versus merely available — is what this cross-reference makes visible,
// and it is decidable from the store alone, with no usage data.
type CrossRef struct {
	// Profiles are the profiles the items were cross-referenced against.
	Profiles []Profile

	// Items holds every item in the store, in load order.
	Items []ItemProfiles
}

// ItemProfiles pairs one item with the names of the profiles selecting it.
type ItemProfiles struct {
	Item store.Item

	// Profiles are the names of the profiles that select this item, in the order
	// the profiles were given. It is empty for a project-local item, which is
	// delivered without being selected at all.
	Profiles []string
}

// AlwaysActive reports whether the item is delivered irrespective of any
// profile. Project-local items are: being local to the project is scope enough,
// so they carry no tags and need no profile to select them.
func (ip ItemProfiles) AlwaysActive() bool {
	return ip.Item.Source == store.OriginLocal
}

// Offered reports whether anything puts this item in front of an agent when a
// session starts — either a profile selecting it, or its being local.
func (ip ItemProfiles) Offered() bool {
	return len(ip.Profiles) > 0 || ip.AlwaysActive()
}

// CrossReference pairs every item in the store with the profiles that select it.
func CrossReference(profiles []Profile, st *store.Store) CrossRef {
	items := make([]ItemProfiles, 0, len(st.All()))
	for _, it := range st.All() {
		ip := ItemProfiles{Item: it}
		// Local items are delivered unconditionally, so asking which profile
		// selects one is not a meaningful question.
		if it.Source == store.OriginStore {
			for _, p := range profiles {
				if selectsItem(p, it) {
					ip.Profiles = append(ip.Profiles, p.Name)
				}
			}
		}
		items = append(items, ip)
	}
	return CrossRef{Profiles: profiles, Items: items}
}

// Names returns the cross-referenced profiles' names, in order.
func (c CrossRef) Names() []string {
	out := make([]string, 0, len(c.Profiles))
	for _, p := range c.Profiles {
		out = append(out, p.Name)
	}
	return out
}

// ByKind returns the cross-referenced items of one kind, in load order.
func (c CrossRef) ByKind(kind store.Kind) []ItemProfiles {
	var out []ItemProfiles
	for _, ip := range c.Items {
		if ip.Item.Kind == kind {
			out = append(out, ip)
		}
	}
	return out
}

// Unoffered returns the items nothing offers at session start, in load order —
// the ones an agent reaches only by searching or listing for them.
func (c CrossRef) Unoffered() []ItemProfiles {
	var out []ItemProfiles
	for _, ip := range c.Items {
		if !ip.Offered() {
			out = append(out, ip)
		}
	}
	return out
}

// selectsItem reports whether a profile's tag selection for the item's kind
// covers that item. It applies exactly the test selectByTags applies when
// composing a session, so the cross-reference cannot claim an item is offered
// that a real session would not deliver.
func selectsItem(p Profile, it store.Item) bool {
	for _, tag := range p.TagsFor(it.Kind) {
		if it.HasTag(tag) {
			return true
		}
	}
	return false
}
