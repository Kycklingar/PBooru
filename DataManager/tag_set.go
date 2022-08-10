package DataManager

import (
	"strconv"
	"strings"

	"github.com/kycklingar/set"
)

func tSetStr(s set.Sorted[Tag]) string {
	return sep(",", len(s.Slice), func(i int) string {
		return strconv.Itoa(s.Slice[i].ID)
	})
}

func lessfnTagID(a, b Tag) bool {
	return a.ID < b.ID
}
func lessfnTag(a, b Tag) bool {
	return a.String() < b.String()
}

func parseTagsWithID(q querier, tagstr string, delim rune) (set.Sorted[Tag], error) {
	return tagChain(parseTags(tagstr, delim)).qids(q).unwrap()
}

func parseTags(tagStr string, delim rune) set.Sorted[Tag] {
	var (
		tagSpitter = make(chan string)
		set        = set.New[Tag](lessfnTag)
	)

	go func(spitter chan string) {
		const (
			next = iota
			unescape
		)

		var (
			state = next
			tag   strings.Builder
		)

		var spit = func() {
			spitter <- strings.ToLower(strings.TrimSpace(tag.String()))
		}

		for _, c := range tagStr {
			switch state {
			case next:
				switch c {
				case '\\':
					state = unescape
				case delim:
					spit()
					tag.Reset()
				default:
					tag.WriteRune(c)
				}
			case unescape:
				tag.WriteRune(c)
				state = next
			}
		}

		if tag.Len() > 0 {
			spit()
		}

		close(spitter)
	}(tagSpitter)

	for tag := range tagSpitter {
		var t Tag
		t.parse(tag)

		// Discard empty tags
		if t.Tag == "" {
			continue
		}

		set.Set(t)
	}

	return set
}
