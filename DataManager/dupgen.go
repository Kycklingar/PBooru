package DataManager

import (
	"encoding/binary"
	"fmt"

	"github.com/Nr90/imgsim"
)

type AppleTree struct {
	Apple *Post
	Pears []*Post
}

func GetAppleTrees(tagStr string) ([]AppleTree, error) {
	tags, err := parseTags(tagStr)
	if err != nil {
		return nil, err
	}


	var join string
	var where string
	if len(tags) > 0 {
		join = `
			JOIN post_tag_mappings ptm0
			ON apple = ptm0.post_id
			` + ptmJoinQuery(tags)

		where = " AND " + ptmWhereQuery(tags)
	}

	query := fmt.Sprintf(`
			SELECT apple, pear
			FROM apple_tree
			WHERE apple IN(
				SELECT apple
				FROM apple_tree
				%s
				WHERE processed IS NULL
				%s
				GROUP BY apple
				ORDER BY apple
				LIMIT 25
			)
			AND processed IS NULL
			ORDER BY apple
			`,
			join,
			where,
	)

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var trees []AppleTree

	putOnTree := func(apple, pear *Post) {
		for i, _ := range trees {
			if trees[i].Apple.ID == apple.ID {
				trees[i].Pears = append(trees[i].Pears, pear)
				return
			}
		}

		trees = append(trees, AppleTree{
			apple,
			[]*Post{pear},
		})
	}

	for rows.Next() {
		var apple, pear = NewPost(), NewPost()

		err = rows.Scan(
			&apple.ID,
			&pear.ID,
		)
		if err != nil {
			return nil, err
		}

		putOnTree(apple, pear)
	}

	return trees, nil
}

func PluckApple(apple int, pears []int) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	for _, pear := range pears {
		_, err := tx.Exec(`
			UPDATE apple_tree
			SET processed = CURRENT_TIMESTAMP
			WHERE apple = $1
			AND pear = $2
			`,
			apple,
			pear,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func GeneratePears() error {
	type Tree struct {
		apple phs
		pears []phs
	}

	type Forest struct {
		trees map[int]Tree
	}

	getForest := func() (Forest, error) {
		var forest = Forest{
			make(map[int]Tree),
		}

		rows, err := DB.Query(`
			SELECT
				p1.post_id, p1.h1, p1.h2, p1.h3, p1.h4,
				p2.post_id, p2.h1, p2.h2, p2.h3, p2.h4
			FROM phash p1
			JOIN phash p2
			ON p1.h1 = p2.h1
			OR p1.h2 = p2.h2
			OR p1.h3 = p2.h3
			OR p1.h4 = p2.h4
			WHERE p1.post_id < p2.post_id
			AND p1.post_id != p2.post_id
			AND p1.post_id IN (
				SELECT post_id
				FROM phash
				WHERE post_id > (
					SELECT coalesce(max(apple), 0)
					FROM apple_tree
				)
				ORDER BY post_id
				LIMIT 10000
			)
			ORDER BY p1.post_id
			`,
		)
		if err != nil {
			return forest, err
		}
		defer rows.Close()

		for rows.Next() {
			var apple, pear phs
			err = rows.Scan(
				&apple.postid,
				&apple.h1,
				&apple.h2,
				&apple.h3,
				&apple.h4,
				&pear.postid,
				&pear.h1,
				&pear.h2,
				&pear.h3,
				&pear.h4,
			)
			if err != nil {
				return forest, err
			}

			tree := forest.trees[apple.postid]

			tree.apple = apple
			tree.pears = append(tree.pears, pear)

			forest.trees[apple.postid] = tree

		}

		return forest, nil
	}

	var err error
	var forest Forest

	for {
		forest, err = getForest()
		if err != nil || len(forest.trees) <= 0 {
			break
		}

		err = func() error {
			tx, err := DB.Begin()
			if err != nil {
				return err
			}

			defer commitOrDie(tx, &err)

			var count int
			for _, tree := range forest.trees {
				count++
				var pears []phs

				// Weed out oranges, we only want to compare apples with pears
				for _, pear := range tree.pears {
					fmt.Printf("[%d/%d] comparing %d %d\n", count, len(forest.trees), tree.apple.postid, pear.postid)
					if tree.apple.distance(pear) < 4 {
						pears = append(pears, pear)
					}
				}

				if err != nil {
					return err
				}

				for _, apple := range pears {
					_, err = tx.Exec(`
						INSERT INTO apple_tree (apple, pear)
						VALUES($1, $2)
						`,
						tree.apple.postid,
						apple.postid,
					)
					if err != nil {
						return err
					}
				}
			}
			return nil
		}()

		if err != nil {
			return err
		}
	}

	return err
}

func generateAppleTree(tx querier, ph phs) error {
	query := func() ([]phs, error) {
		rows, err := tx.Query(`
				SELECT
					p.post_id, p.h1, p.h2, p.h3, p.h4
				FROM phash p
				LEFT JOIN duplicates d
				ON p.post_id = d.dup_id
				WHERE d.post_id IS NULL
				AND p.post_id != $1
				AND (
					p.h1 = $2
					OR p.h2 = $3
					OR p.h3 = $4
					OR p.h4 = $5
				)
			`,
			ph.postid,
			ph.h1,
			ph.h2,
			ph.h3,
			ph.h4,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var hashes []phs

		for rows.Next() {
			var p phs
			err = rows.Scan(
				&p.postid,
				&p.h1,
				&p.h2,
				&p.h3,
				&p.h4,
			)
			if err != nil {
				return nil, err
			}

			hashes = append(hashes, p)
		}

		return hashes, rows.Err()
	}

	hashes, err := query()
	if err != nil {
		return err
	}

	for _, h := range hashes {
		if ph.distance(h) < 4 {
			_, err = tx.Exec(`
				INSERT INTO apple_tree
					(apple, pear)
				VALUES ($1, $2)
				`,
				h.postid,
				ph.postid,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func phsFromHash(postid int, hash imgsim.Hash) phs {
	var ph phs
	ph.postid = postid

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(hash))

	ph.h1 = uint16(b[1]) | uint16(b[0])<<8
	ph.h2 = uint16(b[3]) | uint16(b[2])<<8
	ph.h3 = uint16(b[5]) | uint16(b[4])<<8
	ph.h4 = uint16(b[7]) | uint16(b[6])<<8

	return ph
}

type phs struct {
	postid int
	h1     uint16
	h2     uint16
	h3     uint16
	h4     uint16
}

func (p phs) hash() imgsim.Hash {
	return imgsim.Hash(
		uint64(p.h1)<<16 |
			uint64(p.h2)<<32 |
			uint64(p.h3)<<48 |
			uint64(p.h4)<<64,
	)
}

func (a phs) distance(b phs) int {
	return imgsim.Distance(a.hash(), b.hash())
}

func (a phs) insert(tx querier) error {
	_, err := tx.Exec(`
		INSERT INTO phash (
			post_id, h1, h2, h3, h4
		)
		VALUES (
			$1, $2, $3, $4, $5
		)`,
		a.postid,
		a.h1,
		a.h2,
		a.h3,
		a.h4,
	)

	return err
}
