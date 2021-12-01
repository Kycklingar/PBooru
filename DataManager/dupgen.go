package DataManager

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Nr90/imgsim"
)

const appleTreeDistanceThres = 4

func appleTreeThreshold(dist int) bool {
	return dist <= appleTreeDistanceThres
}

type duration time.Duration

func (d duration) String() string {
	//return time.Duration(d).String()

	days := time.Duration(d).Hours() / 24

	return fmt.Sprintf("%.1f days", days)
}

func (d duration) Value() (driver.Value, error) {
	return driver.Value(int64(d)), nil
}

func (d *duration) Scan(raw interface{}) error {
	switch v := raw.(type) {
	case float64:
		*d = duration(time.Duration(v) * time.Second)
	case nil:
		*d = duration(0)
	default:
		return fmt.Errorf("cannot scan duration from: %#v", v)
	}

	return nil
}

type DupeReportStats struct {
	Average duration
	Total   int
}

func GetDupeReportDelay() (report DupeReportStats, err error) {
	err = DB.QueryRow(`
		SELECT count(*), EXTRACT(EPOCH FROM AVG(approved - timestamp))::float AS delay
		FROM duplicate_report
		WHERE approved IS NOT NULL
		AND report_type = 0
		`,
	).Scan(&report.Total, &report.Average)

	return
}

type AppleTree struct {
	Apple *Post
	Pears []*Post
}

func GetAppleTrees(tagStr string, baseonPear bool, limit, offset int) ([]AppleTree, error) {
	tags, err := parseTagsComics(tagStr)
	if err != nil {
		return nil, err
	}

	var (
		base     = "apple"
		other    = "pear"
		pair     = "apple, pear"
		orderDir = "ASC"
		join     string
		where    string
	)

	if baseonPear {
		base = "pear"
		other = "apple"
		pair = "pear, apple"
		orderDir = "DESC"
	}

	if len(tags) > 0 {
		join = fmt.Sprintf(`
			JOIN post_tag_mappings ptm0
			ON %s = ptm0.post_id
			`+ptmJoinQuery(tags),
			base,
		)

		where = " AND " + ptmWhereQuery(tags)
	}

	query := fmt.Sprintf(`
			WITH reports AS (
				SELECT rep.post_id AS rl, dp.post_id AS rr, report_type
				FROM duplicate_report rep
				JOIN duplicate_report_posts dp
				ON rep.id = dp.report_id
				WHERE approved IS NULL
			)
			
			SELECT %s
			FROM apple_tree
			LEFT JOIN reports
			ON (
				apple = rr
				OR pear = rr
			) AND report_type = 0
			OR (
				apple = rr
				AND pear = rl
			)
			OR (
				apple = rl
				AND pear = rr
			)
			WHERE %s IN(
				SELECT %s
				FROM (
					SELECT %s
					FROM apple_tree
					%s
					LEFT JOIN reports
					ON (
						apple = rr
						OR pear = rr
					) AND report_type = 0
					OR (
						apple = rr
						AND pear = rl
					)
					OR (
						apple = rl
						AND pear = rr
					)
					WHERE processed IS NULL
					AND rl IS NULL
					%s
					GROUP BY %s
					ORDER BY %s %s
				) l
				LEFT JOIN reports
				ON %s = reports.rr
				AND report_type = 0
				WHERE rr IS NULL
				ORDER BY %s %s
				LIMIT $1
				OFFSET $2
			)
			AND rl IS NULL
			AND processed IS NULL
			ORDER BY %s %s, %s
		`,
		pair,
		base,
		base,
		base,
		join,
		where,
		base,
		base, orderDir,
		base,
		base, orderDir,
		base, orderDir, other,
	)

	rows, err := DB.Query(query, limit, offset)
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

func PluckApple(dupe Dupe) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	for _, pear := range dupe.Inferior {
		_, err := tx.Exec(`
			UPDATE apple_tree
			SET processed = CURRENT_TIMESTAMP
			WHERE (
				apple = $1
				AND pear = $2
			) OR (
				apple = $2
				AND pear = $1
			)
			`,
			dupe.Post.ID,
			pear.ID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

type appleTreeHashes struct {
	apple phs
	pears []phs
}

// Remove mismatched appletrees
func RecalculateAppletree() error {
	var applePearHashes = make(map[int]appleTreeHashes)
	err := func() error {
		rows, err := DB.Query(`
		SELECT
			a.post_id,
			a.h1, a.h2,
			a.h3, a.h4,
			b.post_id,
			b.h1, b.h2,
			b.h3, b.h4
		FROM apple_tree
		JOIN phash a
		ON a.post_id = apple
		JOIN phash b
		ON b.post_id = pear
		ORDER BY a.post_id ASC
		`,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				apple phs
				pear  phs
			)

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
				return err
			}

			aph := applePearHashes[apple.postid]
			aph.apple = apple
			aph.pears = append(aph.pears, pear)
			applePearHashes[apple.postid] = aph
		}

		return nil
	}()
	if err != nil {
		return err
	}

	wg := new(sync.WaitGroup)
	errChan := make(chan error)
	ahChan := make(chan appleTreeHashes)

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		DELETE FROM apple_tree
		WHERE apple = $1
		AND pear = $2
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var count int
	length := len(applePearHashes)

	workF := func(ahCh chan appleTreeHashes) {
		defer wg.Done()

		for ahs := range ahCh {
			fmt.Printf("[%d/%d] %d:\t%d\n", count, length, ahs.apple.postid, len(ahs.pears))
			var pears []int
			for _, ph := range ahs.pears {
				// Remove these
				if !appleTreeThreshold(ahs.apple.distance(ph)) {
					pears = append(pears, ph.postid)
				}
			}

			for _, pear := range pears {
				_, err = stmt.Exec(ahs.apple.postid, pear)
				if err != nil {
					errChan <- err
					return
				}

			}
		}
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go workF(ahChan)
	}

	for _, ahash := range applePearHashes {
		count++
		select {
		case err = <-errChan:
			close(ahChan)
			go func() {
				wg.Wait()
				close(errChan)
			}()
			for err2 := range errChan {
				log.Println(err2)
			}
			return err
		case ahChan <- ahash:
			break
		}
	}

	close(ahChan)
	wg.Wait()
	close(errChan)

	tx.Commit()
	return nil
}

func GeneratePears() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	err = GeneratePearsTx(tx)
	if err != nil {
		tx.Rollback()
	} else {
		tx.Commit()
	}

	return err
}

func GeneratePearsTx(tx *sql.Tx) error {
	type Forest map[int]appleTreeHashes

	getForest := func(tx *sql.Tx, start, bufSize int) (Forest, error) {
		rows, err := tx.Query(`
			SELECT
				p1.post_id,
				p1.h1, p1.h2,
				p1.h3, p1.h4,
				p2.post_id,
				p2.h1, p2.h2,
				p2.h3, p2.h4
			FROM phash p1
			JOIN phash p2
			ON p1.h1 = p2.h1
			OR p1.h2 = p2.h2
			OR p1.h3 = p2.h3
			OR p1.h4 = p2.h4
			LEFT JOIN apple_tree
			ON p1.post_id = apple
			AND p2.post_id = pear
			LEFT JOIN duplicates d
			ON d.dup_id = p1.post_id
			OR d.dup_id = p2.post_id
			WHERE p1.post_id < p2.post_id
			AND apple IS NULL
			AND d.post_id IS NULL
			AND p1.post_id > $1
			AND p1.post_id <= $2
			ORDER BY p1.post_id ASC, p2.post_id ASC
			`,
			start,
			start+bufSize,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var forest = make(Forest)

		for rows.Next() {
			var apple, pear phs
			err = rows.Scan(
				&apple.postid,
				&apple.h1, &apple.h2,
				&apple.h3, &apple.h4,
				&pear.postid,
				&pear.h1, &pear.h2,
				&pear.h3, &pear.h4,
			)
			if err != nil {
				return forest, err
			}

			tree := forest[apple.postid]

			tree.apple = apple
			tree.pears = append(tree.pears, pear)

			forest[apple.postid] = tree
		}

		return forest, rows.Err()
	}

	type applepear struct {
		apple phs
		pear  phs
	}

	var (
		maxPost int
		count   int
		bufSize = 10000
		forest  Forest
		err     error

		// Rangers compare the fruit on each tree
		treeChan chan appleTreeHashes

		// Valid apple, pear pairs to be inserted into the db
		pairChan    chan applepear
		rangersWg   = new(sync.WaitGroup)
		ctx, cancel = context.WithCancel(context.Background())
	)
	defer cancel()

	err = tx.QueryRow("SELECT MAX(id) FROM posts").Scan(&maxPost)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO apple_tree(apple, pear)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Rangers calculate the likeness of the fruit and send them out on pairCh
	rangerF := func(ctx context.Context, pairCh chan<- applepear, treeChan <-chan appleTreeHashes, wg *sync.WaitGroup) {
		defer wg.Done()
		for aph := range treeChan {
			for _, pear := range aph.pears {
				if appleTreeThreshold(aph.apple.distance(pear)) {
					select {
					case <-ctx.Done():
						return
					case pairCh <- applepear{apple: aph.apple, pear: pear}:
					}
				}
			}

		}
	}

forest:
	for start := 0; start <= maxPost; start += bufSize {
		fmt.Printf("Gathering forest of %d to %d up until %d, new pairs: %d\n", start, start+bufSize, maxPost, count)
		forest, err = getForest(tx, start, bufSize)
		if err != nil {
			break forest
		}

		pairChan = make(chan applepear, 10)
		treeChan = make(chan appleTreeHashes, 2)
		go func() {
		trees:
			for _, tree := range forest {
				select {
				case <-ctx.Done():
					break trees
				case treeChan <- tree:
				}
			}

			close(treeChan)
			rangersWg.Wait()
			close(pairChan)
		}()

		// Spawn rangers
		for i := 0; i < 50; i++ {
			rangersWg.Add(1)
			go rangerF(ctx, pairChan, treeChan, rangersWg)
		}

		for pair := range pairChan {
			count++
			_, err = stmt.Exec(pair.apple.postid, pair.pear.postid)
			if err != nil {
				break forest
			}
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

	stmt, err := tx.Prepare(`
				INSERT INTO apple_tree
					(apple, pear)
				VALUES ($1, $2)
				`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, h := range hashes {
		if appleTreeThreshold(ph.distance(h)) {
			var apple, pear = h.postid, ph.postid
			if apple > pear {
				apple, pear = pear, apple
			}

			_, err = stmt.Exec(
				apple,
				pear,
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
		uint64(p.h1) |
			uint64(p.h2)<<0x10 |
			uint64(p.h3)<<0x20 |
			uint64(p.h4)<<0x30,
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
