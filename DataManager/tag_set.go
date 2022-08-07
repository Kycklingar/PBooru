package DataManager

import (
	"strconv"
	"strings"
)

type tagSet []*Tag

// sort.Interface implementation
func (set tagSet) Len() int      { return len(set) }
func (set tagSet) Swap(i, j int) { set[i], set[j] = set[j], set[i] }
func (set tagSet) Less(i, j int) bool {
	return set[i].String() < set[j].String()
}

func (set tagSet) strindex(i int) string {
	return strconv.Itoa(set[i].ID)
}

// Return the tag ids in a that are not in b
func (a tagSet) diffID(b tagSet) tagSet {
	var (
		diff = make(map[int]struct{})
		ret  tagSet
	)

	for _, tag := range b {
		diff[tag.ID] = struct{}{}
	}

	for _, tag := range a {
		if _, ok := diff[tag.ID]; !ok {
			ret = append(ret, tag)
		}
	}

	return ret
}

// Return the tags in a that are not in b
func (a tagSet) diff(b tagSet) tagSet {
	var (
		diff = make(map[string]struct{})
		ret  tagSet
	)

	for _, tag := range b {
		diff[tag.String()] = struct{}{}
	}

	for _, tag := range a {
		if _, ok := diff[tag.String()]; !ok {
			ret = append(ret, tag)
		}
	}

	return ret
}

// Return a tagSet with unique ids
func (a tagSet) uniqueID() tagSet {
	m := make(map[int]*Tag)

	for _, t := range a {
		m[t.ID] = t
	}

	var (
		retu = make(tagSet, len(m))
		i    int
	)

	for _, t := range m {
		retu[i] = t
		i++
	}

	return retu
}

// Return a tagSet with unique tags
func (a tagSet) unique() tagSet {
	m := make(map[string]*Tag)

	for _, t := range a {
		m[t.String()] = t
	}

	var (
		retu = make(tagSet, len(m))
		i    int
	)

	for _, t := range m {
		retu[i] = t
		i++
	}

	return retu
}

func parseTagsWithID(q querier, tagstr string, delim rune) (tagSet, error) {
	set := parseTags(tagstr, delim)

	return set.chain().qids(q).unwrap()
}

func parseTags(tagstr string, delim rune) tagSet {
	var (
		tagSpitter = make(chan string)
		set        tagSet
	)

	go func(spitter chan string) {
		const (
			next = iota
			unescape
		)

		var (
			state = next
			tag   string
		)

		for _, c := range tagstr {
			switch state {
			case next:
				switch c {
				case '\\':
					state = unescape
				case delim:
					spitter <- strings.TrimSpace(tag)
					tag = ""
				default:
					tag += string(c)
				}
			case unescape:
				tag += string(c)
				state = next
			}
		}

		spitter <- strings.ToLower(tag)
		close(spitter)
	}(tagSpitter)

	for tag := range tagSpitter {
		var t = NewTag()

		split := strings.SplitN(tag, ":", 2)

		switch len(split) {
		case 1:
			t.Tag = strings.TrimSpace(split[0])
		case 2:
			t.Namespace.Namespace = strings.TrimSpace(split[0])
			t.Tag = strings.TrimSpace(split[1])
		}

		// Discard empty tags
		if t.Tag == "" {
			continue
		}
		if t.Namespace.Namespace == "" {
			t.Namespace.Namespace = "none"
		}

		set = append(set, t)
	}

	return set
}
