package DataManager

import (
	"fmt"
)

var Tombstones int

type Tombstone struct {
	Post    *Post
	E6id    int
	Md5     string
	Reason  string
	Removed timestamp
}

func GetTombstonedPosts(limit, offset int) ([]Tombstone, error) {
	rows, err := DB.Query(`
		SELECT p.id, t.e621_id, reason, t.removed 
		FROM tombstone t
		JOIN hashes h
		ON t.md5 = h.md5
		LEFT JOIN posts p
		ON h.post_id = p.id
		ORDER BY t.removed DESC, p.id DESC
		LIMIT $1
		OFFSET $2
		`,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
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
			return nil, err
		}

		tomb = append(tomb, t)
	}

	return tomb, countTombstones()
}

func countTombstones() error {
	if Tombstones == 0 {
		return DB.QueryRow(`
			SELECT count(*)
			FROM tombstone t
			JOIN hashes h
			ON t.md5 = h.md5
			LEFT JOIN posts p
			ON h.post_id = p.id
			`,
		).Scan(&Tombstones)
	}

	return nil
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
