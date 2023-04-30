package postSearch

import (
	"strconv"

	"github.com/kycklingar/PBooru/DataManager/search"
)

func AltGroup(field search.TableField, group int) altGroup {
	return altGroup{
		field: field,
		group: group,
	}
}

type altGroup struct {
	field search.TableField
	group int
}

func (a altGroup) Condition(w search.Writer) {
	w.WriteString(string(a.field))
	w.WriteString(` = (
				SELECT alt_group
				FROM posts
				WHERE id = `,
	)
	w.WriteString(strconv.Itoa(a.group))
	w.WriteRune(')')
}
