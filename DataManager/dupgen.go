package DataManager

import (
	"github.com/Nr90/imgsim"
)

type Fruit struct {
	Apple *Post
	Pear *Post
}

func GetFruits() ([]Fruit, error){
	rows, err := DB.Query(`
		SELECT apple, pear
		FROM apple_tree
		`,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var fruits []Fruit

	for rows.Next() {
		var fruit = Fruit{
			NewPost(),
			NewPost(),
		}

		err = rows.Scan(
			&fruit.Apple.ID,
			&fruit.Pear.ID,
		)
		if err != nil {
			return nil, err
		}

		fruits = append(fruits, fruit)
	}

	return fruits, nil
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
				LIMIT 100
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

	forest, err := getForest()
	if err != nil {
		return err
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	for _, tree := range forest.trees {
		var pears []phs

		// Weed out oranges, we only want to compare apples with pears
		for _, pear := range tree.pears {
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
