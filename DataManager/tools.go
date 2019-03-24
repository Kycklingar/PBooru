package DataManager

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log"
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
		ids, err := query(tx)
		if err != nil {
			return txError(tx, err)
		}
		if len(ids) <= 0 {
			break
		}

		for _, id := range ids {
			fmt.Printf("Working on id %d hash %s -> ", id.id, id.hash)
			size := ipfsSize(id.hash)
			fmt.Println(size)
			if size <= 0 {
				log.Println("size returned was <= 0, skipping")
				time.Sleep(time.Second)
				continue
				//return txError(tx, errors.New("size returned was <= 0"))
			}

			_, err := tx.Exec("UPDATE posts SET file_size = $1 WHERE id = $2", size, id.id)
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
			file := ipfsCat(p.hash)
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

func MigrateMfs() {
	type post struct {
		Hash string
		ID   int
	}
	query := func(str string, offset int) ([]post, error) {
		rows, err := DB.Query(str, offset*20000)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		defer rows.Close()

		var posts []post

		for rows.Next() {
			var p post
			rows.Scan(&p.Hash, &p.ID)
			posts = append(posts, p)
		}

		return posts, nil
	}

	var err error

	offset := 0
	defer mfsFlush(CFG.MFSRootDir)

	for {
		posts, err := query("SELECT multihash, id FROM posts ORDER BY id ASC LIMIT 20000 OFFSET $1", offset)
		if err != nil || len(posts) <= 0 {
			break
		}
		for _, post := range posts {
			fmt.Printf("Working on file: [%d] %s\n", post.ID, post.Hash)
			if err = mfsCP(CFG.MFSRootDir+"files/", post.Hash, false); err != nil {
				log.Fatal(err)
			}
		}
		offset++
		mfsFlush(CFG.MFSRootDir)
	}
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	offset = 0

	tquery := func(str string, offset int) ([]Thumb, error) {
		rows, err := DB.Query(str, offset*20000)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var thumbs []Thumb
		for rows.Next() {
			var t Thumb
			rows.Scan(&t.Hash, &t.Size)
			thumbs = append(thumbs, t)
		}
		return thumbs, rows.Err()
	}

	for {
		var thumbs []Thumb
		if thumbs, err = tquery("SELECT multihash, dimension FROM thumbnails ORDER BY post_id ASC LIMIT 20000 OFFSET $1", offset); err != nil || len(thumbs) <= 0 {
			break
		}
		for _, thumb := range thumbs {
			fmt.Println("Working on thumbnail:", thumb)
			if err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumb.Size, "/"), thumb.Hash, false); err != nil {
				log.Fatal(err)
			}
		}
		offset++
		mfsFlush(CFG.MFSRootDir)
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

	file := ipfsCat(hash)
	if file == nil {
		return errors.New("File is nil")
	}

	var b bytes.Buffer
	b.ReadFrom(file)

	file.Close()

	f := bytes.NewReader(b.Bytes())

	thumbs, err := makeThumbnails(f)
	if err != nil {
		return err
	}

	if thumbs == nil || len(thumbs) <= 0 {
		return errors.New("no thumbnails returned from makeThumbnails")
	}

	for _, thumb := range thumbs {
		_, err = DB.Exec("INSERT INTO thumbnails(post_id, dimension, multihash) VALUES($1, $2, $3) ON CONFLICT (post_id, dimension) DO UPDATE SET multihash = EXCLUDED.multihash", postID, thumb.Size, thumb.Hash)
		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

func GenerateThumbnails(size int) {

	type P struct {
		id   int
		hash string
	}
	query := func(tx *sql.Tx, offset int) []P {

		rows, err := tx.Query("SELECT p.multihash, p.id FROM posts p LEFT JOIN thumbnails t ON p.id = t.post_id AND t.dimension = $1 WHERE t.post_id IS NULL ORDER BY p.id ASC LIMIT 200 OFFSET $2", size, offset)
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

		hashes := query(tx, failed)

		if len(hashes) <= 0 {
			break
		}

		for _, hash := range hashes {
			fmt.Println("Working on post: ", hash.id, hash.hash)
			file := ipfsCat(hash.hash)
			if file == nil {
				log.Println("File is nil")
				time.Sleep(time.Second)
				failed++
				continue
			}
			var b bytes.Buffer
			b.ReadFrom(file)
			f := bytes.NewReader(b.Bytes())
			thash, err := makeThumbnail(f, size)
			file.Close()
			if err != nil {
				log.Println(err, hash)
				failed++
				continue
			} else if thash == "" {
				log.Println("makeThumbnail did not produce a hash", hash)
				failed++
				continue
			}
			if CFG.UseMFS {
				err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", size, "/"), thash, true)
				if err != nil {
					log.Println(err, thash)
					failed++
					continue
				}
			}
			_, err = tx.Exec("INSERT INTO thumbnails(post_id, dimension, multihash) VALUES($1, $2, $3)", hash.id, size, thash)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
		}
		//mfsFlush(CFG.MFSRootDir)
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

		hashes := query(tx, failed)

		if len(hashes) <= 0 {
			tx.Commit()
			break
		}

		for _, hash := range hashes {
			fmt.Println("Working on post: ", hash.id, hash.hash)
			file := ipfsCat(hash.hash)
			if file == nil {
				log.Println("File is nil")
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

			_, err = tx.Exec("INSERT INTO post_info(post_id, width, height) VALUES($1, $2, $3)", hash.id, width, height)
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