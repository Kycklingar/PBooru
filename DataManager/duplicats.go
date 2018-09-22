package DataManager

import (
	"database/sql"
	"errors"
	C "github.com/kycklingar/PBooru/DataManager/cache"
	"log"
	"strconv"
)

func NewDuplicate() *Duplicate {
	return &Duplicate{Post: NewPost()}
}

type Duplicate struct {
	ID    int
	DupID int
	Post  *Post
	Level int

	Posts []*Post
}

func (d *Duplicate) QID(q querier) int {
	if d.ID != 0 {
		return d.ID
	}

	if d.Post.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT id FROM duplicate_posts WHERE post_id=?", d.Post.QID(q)).Scan(&d.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return 0
	}

	return d.ID
}

func (d *Duplicate) QDupID(q querier) int {
	if d.DupID != 0 {
		return d.DupID
	}

	if d.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT dup_id FROM duplicate_posts WHERE id=?", d.QID(q)).Scan(&d.DupID)
	if err != nil {
		log.Print(err)
		return 0
	}

	return d.DupID
}

func (d *Duplicate) SetDupID(q querier, id int) error {
	err := q.QueryRow("SELECT dup_id FROM duplicate_posts WHERE dup_id=?", id).Scan(&d.DupID)
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

func (d *Duplicate) QLevel(q querier) int {
	if d.Level != 0 {
		return d.Level
	}

	if d.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT level FROM duplicate_posts WHERE id=?", d.QID(q)).Scan(&d.Level)
	if err != nil {
		log.Print(err)
	}

	return d.Level
}

func (d *Duplicate) BestPost(q querier) *Post {
	if d.Post.QID(q) == 0 {
		return d.Post
	}
	if d.QID(q) == 0 {
		return d.Post
	}
	ps := d.QPosts(q)
	bdp := d
	for _, p := range ps {
		dup := NewDuplicate()
		dup.Post = p
		//dup.setQ(q)

		if dup.QLevel(q) < bdp.QLevel(q) {
			bdp = dup
		}
	}
	return bdp.Post
}

func (d *Duplicate) QPosts(q querier) []*Post {
	if d.Posts != nil {
		return d.Posts
	}

	if d.QDupID(q) == 0 {
		d.Posts = []*Post{}
		return nil
	}

	rows, err := q.Query("SELECT post_id FROM duplicate_posts WHERE dup_id=? ORDER BY level", d.QDupID(q))
	if err != nil {
		log.Print(err)
		return nil
	}

	defer rows.Close()

	for rows.Next() {
		p := NewPost()

		err = rows.Scan(&p.ID)
		if err != nil {
			log.Print(err)
			return nil
		}
		d.Posts = append(d.Posts, p)
	}

	return d.Posts
}

func (d *Duplicate) Save() error {
	if d.Post.QID(DB) == 0 {
		return errors.New("post doesn't exist")
	}

	if d.QLevel(DB) < 1 {
		return errors.New("level must be > 0")
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Print(err)
		return err
	}
	//d.setQ(tx)

	if d.QDupID(tx) == 0 {
		var tmp *int
		err = tx.QueryRow("SELECT MAX(dup_id) FROM duplicate_posts").Scan(&tmp)
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}
		if tmp == nil {
			d.DupID = 1
		} else {
			d.DupID = *tmp + 1
		}
	}

	if d.QID(tx) == 0 {
		//return errors.New("dup already exist")
		_, err = tx.Exec("INSERT INTO duplicate_posts(dup_id, post_id, level) VALUES(?, ?, ?)", d.QDupID(tx), d.Post.QID(tx), d.QLevel(tx))
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}
	} else {
		_, err = tx.Exec("UPDATE duplicate_posts SET dup_id = ?, level = ? WHERE id=?", d.QDupID(tx), d.QLevel(tx), d.QID(tx))
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}
	}

	ps := d.QPosts(tx)

	lp := d
	// Find the lowset leveled duplicate and
	// replace all tags, comics with this post
	for _, p := range ps {
		a := NewDuplicate()
		//a.setQ(tx)

		a.Post = p

		if a.QLevel(tx) < 1 {
			return txError(tx, errors.New("a.level is below zero"))
		}

		if a.QLevel(tx) < lp.QLevel(tx) {
			lp = a
		}
	}

	ps = append(ps, d.Post)

	for _, p := range ps {
		//p.setQ(tx)
		//fmt.Println(p.ID(tx), lp.Post.ID(tx))
		// Don't want the superior to owerwrite itself
		if p.QID(tx) == lp.Post.QID(tx) {
			continue
		}

		// Collect all tags from all posts in this duplicate and add them to the superior post
		var tc TagCollector
		err := tc.GetFromPost(tx, *p)
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}

		err = tc.AddToPost(tx, lp.Post)
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}

		// Remove the tags from the less superior
		err = tc.RemoveFromPost(tx, p)
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}

		// Replace all comics with the superior
		cp := newComicPost()
		//cp.SetQ(tx)
		cp.Post = p

		err = cp.replacePost(tx, lp.Post)
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}

		// Lastly delete the inferior dupe
		err = p.Delete(tx)
		if err != nil {
			log.Print(err)
			return txError(tx, err)
		}
	}

	lp.Post.UnDelete(tx)

	err = tx.Commit()
	if err != nil {
		log.Print(err)
		return err
	}

	//d.setQ(nil)

	for _, p := range d.QPosts(DB) {
		//fmt.Println(p.ID())
		C.Cache.Purge("TC", strconv.Itoa(p.QID(DB)))
	}
	C.Cache.Purge("TC", strconv.Itoa(d.Post.QID(DB)))
	return err
}
