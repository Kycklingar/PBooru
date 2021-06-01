package DataManager

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kycklingar/PBooru/DataManager/image"
	ipfsdir "github.com/kycklingar/PBooru/DataManager/ipfs-dirgen"
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
		rows, err := q.Query("SELECT id, multihash FROM posts WHERE file_size = 0 AND deleted IS FALSE LIMIT $1", limit)
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
		rows, err := tx.Query("SELECT p.id, p.multihash FROM posts p LEFT JOIN hashes h ON p.id = h.post_id WHERE h.post_id IS NULL AND p.deleted IS FALSE ORDER BY p.id LIMIT $1", limit)
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

func InitDir() {
	var items, processed, top int

	err := DB.QueryRow(`
		SELECT MAX(id)
		FROM posts
		`,
	).Scan(&top)

	rows, err := DB.Query(`
		SELECT multihash
		FROM posts
		WHERE deleted IS FALSE
		`,
	)
	if err != nil {
		log.Fatal(err)
	}

	var pmap = make(map[string][]string)

	for rows.Next() {
		items++
		var cid string
		if err = rows.Scan(&cid); err != nil {
			log.Fatal(err)
		}

		cids := pmap[cidDir(cid)]
		cids = append(cids, cid)

		pmap[cidDir(cid)] = cids
	}
	rows.Close()

	rows, err = DB.Query(`
		SELECT dimension, t.multihash, p.multihash
		FROM thumbnails t
		JOIN posts p
		ON t.post_id = p.id
		`,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Map of sizes holding ciddir [thumb,post]
	var tmaps = make(map[int]map[string][][]string)

	for rows.Next() {
		items++
		var (
			size int
			tcid string
			pcid string
		)

		err = rows.Scan(&size, &tcid, &pcid)
		if err != nil {
			log.Fatal(err)
		}

		tm, ok := tmaps[size]
		if !ok {
			tm = make(map[string][][]string)
		}

		cids := tm[cidDir(pcid)]
		cids = append(cids, []string{tcid, pcid})

		tm[cidDir(pcid)] = cids
		tmaps[size] = tm
	}
	rows.Close()

	type file struct {
		name string
		cid  string
		size uint64
		dim  int
	}

	statloop := func(wg *sync.WaitGroup, in, out chan file) {
		defer wg.Done()
		for f := range in {
			stat, err := ipfs.FilesStat(context.Background(), "/ipfs/"+f.cid)
			if err != nil {
				log.Fatal(err)
			}
			f.size = stat.Size

			out <- f
		}
	}

	var (
		in   = make(chan file, 10)
		out  = make(chan file)
		root = ipfsdir.NewDir("")
		wg   sync.WaitGroup
	)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go statloop(&wg, in, out)
	}

	go func() {
		for _, v := range pmap {
			for _, cid := range v {
				in <- file{name: cid, cid: cid}
			}
		}
		close(in)
	}()

	done := make(chan bool)

	go func(done chan bool) {
		filesDir := root.AddDir(ipfsdir.NewDir("files"))
		fdirm := make(map[string]*ipfsdir.Dir)
		for {
			select {
			case <-done:
				return
			case f := <-out:
				{
					dir := cidDir(f.cid)
					d, ok := fdirm[dir]
					if !ok {
						d = filesDir.AddDir(ipfsdir.NewDir(dir))
						fdirm[dir] = d
					}

					processed++
					fmt.Printf("[%f%%] Adding to %s %s %d\n", float32(processed)/float32(items)*100, dir, f.cid, f.size)
					err = d.AddLink(f.cid, f.cid, f.size)
					if err != nil {
						log.Fatal(err)
					}
				}

			}
		}
	}(done)

	wg.Wait()
	done <- true

	//filesDir := root.AddDir(ipfsdir.NewDir("files"))
	//for k, v := range pmap {
	//	d := filesDir.AddDir(ipfsdir.NewDir(k))
	//	for c, cid := range v {
	//		var percent = (float32(i)/float32(len(pmap)) + (float32(c)/float32(len(v)))/float32(len(pmap))) * 100

	//		stat, err := ipfs.FilesStat(context.Background(), "/ipfs/"+cid)
	//		if err != nil {
	//			log.Fatal(err)
	//		}

	//		fmt.Printf("[%f%%] Adding to %s %s %d\n", percent, k, cid, stat.Size)
	//		err = d.AddLink(cid, cid, stat.Size)
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//	}
	//	i++

	//	root.AddDir(d)
	//}

	in = make(chan file, 10)
	out = make(chan file)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go statloop(&wg, in, out)
	}

	go func() {
		for dim, m := range tmaps {
			for _, cids := range m {
				for _, cid := range cids {
					in <- file{name: cid[1], cid: cid[0], dim: dim}
				}
			}
		}
		close(in)
	}()

	go func(done chan bool) {
		tdirs := root.AddDir(ipfsdir.NewDir("thumbnails"))
		tdirm := make(map[int]map[string]*ipfsdir.Dir)
		sdirm := make(map[int]*ipfsdir.Dir)
		for {
			select {
			case <-done:
				return
			case f := <-out:
				{
					sizedir, ok := sdirm[f.dim]
					if !ok {
						sizedir = tdirs.AddDir(ipfsdir.NewDir(strconv.Itoa(f.dim)))
						sdirm[f.dim] = sizedir
						tdirm[f.dim] = make(map[string]*ipfsdir.Dir)
					}

					ciddir := cidDir(f.name)

					tdir, ok := tdirm[f.dim][ciddir]
					if !ok {
						tdir = sizedir.AddDir(ipfsdir.NewDir(ciddir))
						tdirm[f.dim][ciddir] = tdir
					}

					processed++
					fmt.Printf("[%f%%] Adding to %d %s %s %s %d\n", float32(processed)/float32(items)*100, f.dim, ciddir, f.name, f.cid, f.size)
					err = tdir.AddLink(f.name, f.cid, f.size)
					if err != nil {
						log.Fatal(err)
					}
				}

			}
		}
	}(done)

	wg.Wait()
	done <- true

	//thumbDir := root.AddDir(ipfsdir.NewDir("thumbnails"))
	//for size, m := range tmaps {
	//	sizeDir := thumbDir.AddDir(ipfsdir.NewDir(strconv.Itoa(size)))
	//	var i int
	//	for dir, cids := range m {
	//		d := sizeDir.AddDir(ipfsdir.NewDir(dir))

	//		for c, cid := range cids {
	//			var percent = (float32(i)/float32(len(m)) + (float32(c)/float32(len(cids)))/float32(len(m))) * 100

	//			stat, err := ipfs.FilesStat(context.Background(), "/ipfs/"+cid[0])
	//			if err != nil {
	//				log.Fatal(err)
	//			}

	//			fmt.Printf("[%f%%] Adding to %d %s %s %s %d\n", percent, size, dir, cid[0], cid[1], stat.Size)
	//			err = d.AddLink(cid[1], cid[0], stat.Size)
	//		}
	//	}
	//	i++
	//}

	cid, _, err := root.Put(ipfs)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Top", top)
	fmt.Println(cid)
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
				AND deleted IS FALSE
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
	err := DB.QueryRow("SELECT multihash FROM posts WHERE id = $1 AND deleted IS FALSE", postID).Scan(&hash)
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

		rows, err := tx.Query("SELECT p.multihash, p.id FROM posts p LEFT JOIN thumbnails t ON p.id = t.post_id AND t.dimension = $1 WHERE t.post_id IS NULL AND p.deleted IS FALSE ORDER BY p.id ASC LIMIT 200 OFFSET $2", size, offset)
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

		rows, err := tx.Query("SELECT p.multihash, p.id FROM posts p LEFT JOIN post_info t ON p.id = t.post_id WHERE t.post_id IS NULL AND p.deleted IS FALSE ORDER BY p.id ASC LIMIT 200 OFFSET $1", offset)
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

func GenPhash() error {
	query := func(tx *sql.Tx, skip int) ([]*Post, error) {
		rows, err := tx.Query(`
			SELECT p.id, p.multihash
			FROM posts p
			LEFT JOIN phash ph
			ON p.id = ph.post_id
			WHERE ph.post_id IS NULL
			ORDER BY p.id
			LIMIT 200
			OFFSET $1
			`,
			skip,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var posts []*Post

		for rows.Next() {
			var p Post
			err = rows.Scan(&p.ID, &p.Hash)
			if err != nil {
				return nil, err
			}
			posts = append(posts, &p)
		}

		return posts, nil
	}

	var (
		tx     *sql.Tx
		err    error
		failed int
		batch  []*Post
	)

	for {
		tx, err = DB.Begin()
		if err != nil {
			return err
		}

		batch, err = query(tx, failed)
		if len(batch) <= 0 || err != nil {
			tx.Commit()
			break
		}

		for _, post := range batch {
			fmt.Println("Working on ", post.ID, post.Hash)
			f, err := ipfs.Cat(post.Hash)
			if err != nil {
				tx.Rollback()
				return err
			}

			var b bytes.Buffer
			b.ReadFrom(f)
			r := bytes.NewReader(b.Bytes())
			f.Close()

			u, err := dHash(r)
			if err != nil {
				log.Println(err)
				failed++
				continue
			}

			if u > 0 {
				ph := phsFromHash(post.ID, u)
				err = ph.insert(tx)
				if err != nil {
					tx.Rollback()
					return err
				}

				err = generateAppleTree(tx, ph)
				if err != nil {
					tx.Rollback()
					return err
				}
			} else {
				fmt.Println(u)
				failed++
			}
		}

		tx.Commit()
	}

	return err
}

func UpdateUserFlags(newFlag, oldFlag int) error {
	_, err := DB.Exec("UPDATE users SET adminflag = $1 WHERE adminflag = $2", newFlag, oldFlag)
	return err
}

func UpdateTombstone(filep string) error {
	f, err := os.Open(filep)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var tombs []Tombstone

	for scanner.Scan() {
		line := scanner.Text()

		str := strings.SplitN(line, "\t", 5)
		if len(str) != 5 {
			if len(tombs) > 0 {
				tombs[len(tombs)-1].Reason += line
			}
			continue
		}

		id, err := strconv.Atoi(str[0])
		if err != nil {
			return err
		}

		var t time.Time

		// Check missing flag
		if str[4] == "Flag missing" {
			t = time.Unix(0, 0)
		} else {
			t, err = time.Parse(time.RFC3339, str[3])
			if err != nil {
				return err
			}
		}

		var ts = timestamp{&t}

		var tomb = Tombstone{
			E6id:    id,
			Md5:     str[1],
			Reason:  str[4],
			Removed: ts,
		}

		tombs = append(tombs, tomb)
	}

	return insertTombstones(tombs)
}

func VerifyFileIntegrity(start int) error {
	rows, err := DB.Query("SELECT multihash FROM posts WHERE deleted = false AND id > $1 ORDER BY id ASC", start)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid string

		if err = rows.Scan(&cid); err != nil {
			return err
		}

		fmt.Print(cid, ":[")

		ch, err := ipfs.Refs(cid, true)
		if err != nil {
			return err
		}

		for range ch {
			fmt.Print("x")
		}
		fmt.Print("]\n")
	}

	return nil
}
