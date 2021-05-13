package DataManager

import (
	"fmt"

	c "github.com/kycklingar/PBooru/DataManager/cache"
)

	c "github.com/kycklingar/PBooru/DataManager/cache"
)

type Tombstone struct {
	Post    *Post
	E6id    int
	Md5     string
	Reason  string
	Removed timestamp
}

func GetTombstonedPosts(query string, limit, offset int) (int, []Tombstone, error) {
	var where string
	var data []interface{}
	var pos int
	if query != "" {
		where = "WHERE reason LIKE('%'||$1||'%')"
		data = append(data, query)
		pos = 1
	}

	data = append(data, []interface{}{limit, offset}...)

	var from = fmt.Sprintf(`
			FROM tombstone t
			JOIN posts p
			ON t.post_id = p.id
			%s
		`,
		where,
	)

	var q = fmt.Sprintf(`
		SELECT p.id, t.e621_id, reason, t.removed 
		%s
		ORDER BY t.removed DESC, p.id DESC
		LIMIT $%d
		OFFSET $%d
		`,
		from,
		pos+1,
		pos+2,
	)

	rows, err := DB.Query(q, data...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var tomb []Tombstone

	for rows.Next() {
		var t = Tombstone{
			Post: NewPost(),
		}

		if err = rows.Scan(
			&t.Post.ID,
			&t.E6id,
			&t.Reason,
			&t.Removed,
		); err != nil {
			return 0, nil, err
		}

		tomb = append(tomb, t)
	}

	total, err := countTombstones(from, query)

	return total, tomb, err
}

func countTombstones(query, param string) (int, error) {
	if t := c.Cache.Get("TOMB", param); t != nil {
		return t.(int), nil
	}

	var data []interface{}
	if param != "" {
		data = append(data, param)
	}

	var total int
	err := DB.QueryRow(
		fmt.Sprintf(`
			SELECT count(*)
			%s
			`,
			query,
		),
		data...,
	).Scan(&total)

	c.Cache.Set("TOMB", param, total)

	return total, err
}

func insertTombstones(tombs []Tombstone) error {
	stmt, err := DB.Prepare(
		`INSERT INTO tombstone (
			e621_id,
			md5,
			reason,
			removed
		)
		VALUES($1, $2, $3, $4
		)
		ON CONFLICT DO NOTHING`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range tombs {
		fmt.Println(t)
		if _, err = stmt.Exec(
			t.E6id,
			t.Md5,
			t.Reason,
			*t.Removed.Time(),
		); err != nil {
			return err
		}
	}

	return nil
}
