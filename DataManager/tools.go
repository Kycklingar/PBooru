package DataManager

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/kycklingar/PBooru/DataManager/image"
)

func GenerateFileSizes() error {
	type p struct {
		id   int
		hash string
	}

	var tx, err = DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}

	query := func(q querier) ([]p, error) {
		limit := 500
		rows, err := q.Query("SELECT id, multihash FROM posts WHERE file_size = 0 LIMIT $1", limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var ids []p
		for rows.Next() {
			var id p
			err := rows.Scan(&id.id, &id.hash)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, nil
	}

	for {
		posts, err := query(tx)
		if err != nil {
			return txError(tx, err)
		}
		if len(posts) <= 0 {
			break
		}

		for _, post := range posts {
			fmt.Printf("Working on id %d hash %s -> ", post.id, post.hash)
			size, err := ipfsSize(post.hash)
			if err != nil {
				log.Println(err)
				continue
			}
			fmt.Println(size)
			if size <= 0 {
				log.Println("size returned was <= 0, skipping")
				//time.Sleep(time.Second)
				continue
			}

			_, err = tx.Exec("UPDATE posts SET file_size = $1 WHERE id = $2", size, post.id)
			if err != nil {
				log.Println(err)
				return txError(tx, err)
			}
		}

		tx.Commit()
		tx, err = DB.Begin()
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return tx.Commit()
}

func CalculateChecksums() error {
	type P struct {
		id   int
		hash string
	}

	var tx, err = DB.Begin()
	if err != nil {
		return err
	}

	query := func() ([]P, error) {
		var limit = 5000
		rows, err := tx.Query("SELECT p.id, p.multihash FROM posts p LEFT JOIN hashes h ON p.id = h.post_id WHERE h.post_id IS NULL ORDER BY p.id LIMIT $1", limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var posts []P
		for rows.Next() {
			var p P
			err = rows.Scan(&p.id, &p.hash)
			if err != nil {
				return nil, err
			}
			posts = append(posts, p)
		}

		return posts, rows.Err()
	}

	for {
		posts, err := query()
		if err != nil {
			return txError(tx, err)
		}
		if len(posts) <= 0 {
			break
		}

		for _, p := range posts {
			fmt.Printf("Calculating checksum on post: %d %s\n", p.id, p.hash)
			file, err := ipfsCat(p.hash)
			if err != nil {
				log.Println(err)
				continue
			}
			sha, md := checksum(file)
			file.Close()

			if sha == "" || md == "" {
				return txError(tx, errors.New("Checksum empty"))
			}

			_, err = tx.Exec("INSERT INTO hashes(post_id, sha256, md5) VALUES($1, $2, $3)", p.id, sha, md)
			if err != nil {
				return txError(tx, err)
			}
		}

		tx.Commit()
		tx, err = DB.Begin()
		if err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

func MigrateStore(start int) {
	type thumb struct {
		size int
		cid  string
	}
	type post struct {
		cid    string
		id     int
		thumbs []thumb
	}

	query := func(str string, start, offset int) ([]post, error) {
		rows, err := DB.Query(str, start, offset*2000)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		defer rows.Close()

		var postm = make(map[int]post)

		for rows.Next() {
			var (
				p     post
				tsize sql.NullInt64
				tcid  sql.NullString
			)

			err = rows.Scan(&p.id, &p.cid, &tsize, &tcid)
			if err != nil {
				return nil, err
			}

			if _, ok := postm[p.id]; !ok {
				postm[p.id] = p
			} else {
				p = postm[p.id]
			}

			if tsize.Valid && tcid.Valid {
				p.thumbs = append(
					p.thumbs,
					thumb{
						int(tsize.Int64),
						tcid.String,
					},
				)

				postm[p.id] = p
			}
		}

		var posts = make([]post, len(postm))
		var i int

		for _, v := range postm {
			posts[i] = v
			i++
		}

		sort.Slice(posts, func(i, j int) bool {
			return posts[i].id < posts[j].id
		})

		return posts, nil
	}

	var err error

	offset := 0

	for {
		posts, err := query(`
			SELECT p.id, p.multihash, t.dimension, t.multihash
			FROM thumbnails t
			RIGHT JOIN (
				SELECT id, multihash
				FROM posts
				WHERE id > $1
				ORDER BY id ASC
				LIMIT 2000
				OFFSET $2
			) AS p
			ON p.id = t.post_id
			`,
			start,
			offset,
		)
		if err != nil || len(posts) <= 0 {
			break
		}

		for _, p := range posts {
			fmt.Printf("Working on file: [%d] %s\n", p.id, p.cid)
			if err = store.Store(
				p.cid,
				storeFileDest(p.cid),
			); err != nil {
				log.Fatal(err)
			}

			for _, t := range p.thumbs {
				fmt.Printf("\tthumbnail: [%d] %s\n", t.size, t.cid)
				if err = store.Store(
					t.cid,
					storeThumbnailDest(p.cid, t.size),
				); err != nil {
					log.Fatal(err)
				}
			}
		}
		offset++
	}
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

}

func GenerateThumbnail(postID int) error {
	var hash string
	err := DB.QueryRow("SELECT multihash FROM posts WHERE id = $1", postID).Scan(&hash)
	if err != nil {
		log.Println(err)
		return err
	}

	file, err := ipfsCat(hash)
	if err != nil {
		return err
	}

	var b = new(bytes.Buffer)
	b.ReadFrom(file)

	f := bytes.NewReader(b.Bytes())

	thumbs, err := makeThumbnails(f)
	if err != nil {
		return err
	}

	if thumbs == nil || len(thumbs) <= 0 {
		return errors.New("no thumbnails returned from makeThumbnails")
	}

	for _, thumb := range thumbs {
		tx, err := DB.Begin()
		if err != nil {
			log.Println(err)
			continue
		}

		_, err = tx.Exec("INSERT INTO thumbnails(post_id, dimension, multihash) VALUES($1, $2, $3) ON CONFLICT (post_id, dimension) DO UPDATE SET multihash = EXCLUDED.multihash", postID, thumb.Size, thumb.Hash)
		if err != nil {
			log.Println(err)
			tx.Rollback()
			continue
		}

		if err = store.Store(thumb.Hash, storeThumbnailDest(hash, thumb.Size)); err != nil {
			log.Println(err)
			tx.Rollback()
			continue
		}

		tx.Commit()
	}

	return nil
}

func GenerateThumbnails(size int) {

	type P struct {
		id   int
		hash string
	}
	query := func(tx *sql.Tx, offset int) []P {

		rows, err := tx.Query("SELECT p.multihash, p.id FROM posts p LEFT JOIN thumbnails t ON p.id = t.post_id AND t.dimension = $1 WHERE t.post_id IS NULL AND p.deleted = false ORDER BY p.id ASC LIMIT 200 OFFSET $2", size, offset)
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
		defer rows.Close()

		var hashes []P

		for rows.Next() {
			var p P
			err = rows.Scan(&p.hash, &p.id)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
			hashes = append(hashes, p)
		}
		return hashes
	}

	var failed int
	for {
		var tx, err = DB.Begin()
		if err != nil {
			log.Fatal(err)
		}

		posts := query(tx, failed)

		if len(posts) <= 0 {
			break
		}

		for _, post := range posts {
			fmt.Println("Working on post: ", post.id, post.hash)
			file, err := ipfsCat(post.hash)
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second)
				failed++
				continue
			}
			var b bytes.Buffer
			b.ReadFrom(file)
			f := bytes.NewReader(b.Bytes())
			thash, err := makeThumbnail(f, size, CFG.ThumbnailQuality)
			file.Close()
			if err != nil {
				log.Println(err, post)
				failed++
				continue
			} else if thash == "" {
				log.Println("makeThumbnail did not produce a hash", post)
				failed++
				continue
			}

			err = store.Store(thash, storeThumbnailDest(post.hash, size))
			if err != nil {
				log.Println(err, thash)
				failed++
				continue
			}

			_, err = tx.Exec("INSERT INTO thumbnails(post_id, dimension, multihash) VALUES($1, $2, $3)", post.id, size, thash)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
		}
		//mfsFlush(CFG.MFSRootDir)
		tx.Commit()
	}
}

func UpgradePostCids() {
	fmt.Println("Upgrading post cids to base32")

	type P struct {
		id   int
		hash string
	}

	query := func(tx *sql.Tx) []P {
		rows, err := tx.Query("SELECT id, multihash FROM posts WHERE LENGTH(multihash) < 50 ORDER BY id ASC LIMIT 10000")
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
		defer rows.Close()

		var posts []P

		for rows.Next() {
			var p P
			err = rows.Scan(&p.id, &p.hash)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}

			posts = append(posts, p)
		}

		return posts
	}

	for {
		var tx, err = DB.Begin()
		if err != nil {
			log.Fatal(err)
		}

		posts := query(tx)
		if len(posts) <= 0 {
			fmt.Println("All done!")
			tx.Commit()
			break
		}

		for _, post := range posts {

			fmt.Print("Working on ", post.id, " ", post.hash, " -> ")
			base32, err := ipfsUpgradeCidBase32(post.hash)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}

			fmt.Println(base32)

			_, err = tx.Exec("UPDATE posts SET multihash = $1 WHERE id =$2", base32, post.id)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}

		}

		tx.Commit()
	}
}

func GenerateFileDimensions() {
	type P struct {
		id   int
		hash string
	}
	query := func(tx *sql.Tx, offset int) []P {

		rows, err := tx.Query("SELECT p.multihash, p.id FROM posts p LEFT JOIN post_info t ON p.id = t.post_id WHERE t.post_id IS NULL ORDER BY p.id ASC LIMIT 200 OFFSET $1", offset)
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
		defer rows.Close()

		var hashes []P

		for rows.Next() {
			var p P
			err = rows.Scan(&p.hash, &p.id)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
			hashes = append(hashes, p)
		}
		return hashes
	}

	var failed int
	for {
		var tx, err = DB.Begin()
		if err != nil {
			log.Fatal(err)
		}

		posts := query(tx, failed)

		if len(posts) <= 0 {
			tx.Commit()
			break
		}

		for _, post := range posts {
			fmt.Println("Working on post: ", post.id, post.hash)
			file, err := ipfsCat(post.hash)
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second)
				failed++
				continue
			}

			width, height, err := image.GetDimensions(file)
			file.Close()
			if err != nil {
				log.Println(err)
				failed++
				continue
			}

			if width == 0 || height == 0 {
				log.Println("width or height <= 0: ", width, height)
				failed++
				continue
			}

			_, err = tx.Exec("INSERT INTO post_info(post_id, width, height) VALUES($1, $2, $3)", post.id, width, height)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
		}
		tx.Commit()
	}
}

func UpdateUserFlags(newFlag, oldFlag int) error {
	_, err := DB.Exec("UPDATE users SET adminflag = $1 WHERE adminflag = $2", newFlag, oldFlag)
	return err
}
