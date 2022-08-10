package DataManager

import (
	"fmt"
)

//TODO remove file / move contents

const (
	cacheComic = "CMC"
)

func ptmWhereQuery(tags []Tag) string {
	if len(tags) <= 0 {
		return ""
	}

	var ptmWhere string

	for i := 0; i < len(tags)-1; i++ {
		ptmWhere += fmt.Sprintf(
			"ptm%d.tag_id = %d AND ",
			i,
			tags[i].ID,
		)
	}

	ptmWhere += fmt.Sprintf(
		"ptm%d.tag_id = %d",
		len(tags)-1,
		tags[len(tags)-1].ID,
	)

	return ptmWhere
}

func ptmJoinQuery(tags []Tag) string {
	if len(tags) < 2 {
		return ""
	}

	var ptmJoin string
	for i := 0; i < len(tags)-1; i++ {
		ptmJoin += fmt.Sprintf(`
			JOIN post_tag_mappings ptm%d
			ON ptm%d.post_id = ptm%d.post_id
			`,
			i+1,
			i,
			i+1,
		)
	}

	return ptmJoin
}
