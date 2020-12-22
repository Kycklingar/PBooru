package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/querier"
)

func NewChapter() *Chapter {
	return &Chapter{Order: -9999999999}
}

func newChapter() *Chapter {
	var c Chapter
	c.Comic = NewComic()
	return &c
}

type Chapter struct {
	ID        int
	Comic     *Comic
	Title     string
	Order     int
	PageCount int

	Posts []*ComicPost
}

func (c *Chapter) ShiftPosts(user *User, symbol, page, by int) error {
	symb := func() string {
		if symbol > 0 {
			return ">"
		}
		return "<"
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	query := fmt.Sprintf(`
		UPDATE comic_mappings
		SET post_order = post_order + $1
		WHERE chapter_id = $2
		AND post_order %s $3
		RETURNING id, post_id, post_order
		`,
		symb(),
	)

	comicPosts, err := func() ([]*ComicPost, error) {
		rows, err := tx.Query(query, by, c.ID, page)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var comicPosts []*ComicPost

		for rows.Next() {
			var cp = newComicPost()
			err = rows.Scan(&cp.ID, &cp.Post.ID, &cp.Order)
			if err != nil {
				return nil, err
			}

			cp.Chapter = c

			comicPosts = append(comicPosts, cp)
		}

		return comicPosts, nil
	}()
	if err != nil {
		return err
	}

	for _, cp := range comicPosts {
		err = cp.log(tx, lUpdate, user)
		if err != nil {
			return err
		}
	}

	err = c.QComic(tx)
	if err != nil {
		return err
	}

	C.Cache.Purge(cacheComic, strconv.Itoa(c.Comic.ID))

	return nil
}

func (c *Chapter) QID(q querier.Q) int {
	if c.ID != 0 {
		return c.ID
	}
	if c.Comic.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT id FROM comic_Chapter WHERE comic_id=$1 AND c_order=$2", c.Comic.QID(q), c.QOrder(q)).Scan(&c.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
	}
	return c.ID
}

func (c *Chapter) SetID(id int) {
	c.ID = id
}

func (c *Chapter) QComic(q querier.Q) error {
	if c.Comic != nil {
		return nil
	}

	var comic = new(Comic)
	err := q.QueryRow("SELECT comic_id FROM comic_chapter WHERE id = $1", c.ID).Scan(&comic.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println(err)
		}
		return err
	}

	c.Comic = comic
	return nil
}

func (c *Chapter) QTitle(q querier.Q) string {
	if c.Title != "" {
		return c.Title
	}
	if c.QID(q) == 0 {
		return ""
	}

	err := q.QueryRow("SELECT title FROM comic_Chapter WHERE id=$1", c.QID(q)).Scan(&c.Title)
	if err != nil {
		log.Print(err)
	}

	return c.Title
}

func (c *Chapter) QOrder(q querier.Q) int {
	if c.Order > -999999 {
		return c.Order
	}
	if c.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT c_order FROM comic_Chapter WHERE id=$1", c.QID(q)).Scan(&c.Order)
	if err != nil {
		log.Print(err)
	}

	return c.Order
}

func (c *Chapter) QPageCount(q querier.Q) int {
	if c.PageCount > 0 {
		return c.PageCount
	}
	c.PageCount = len(c.Posts)
	if c.PageCount > 0 {
		return c.PageCount
	}

	if err := q.QueryRow("SELECT count(*) FROM comic_mappings WHERE chapter_id = $1", c.QID(q)).Scan(&c.PageCount); err != nil {
		log.Println(err)
		return 0
	}

	return c.PageCount
}

func (c *Chapter) Save(q querier.Q, user *User) error {
	if c.QID(q) != 0 {
		return errors.New("Chapter already exist")
	}

	if c.Comic.QID(q) == 0 {
		return errors.New("comic id not set")
	}

	var count int

	if err := q.QueryRow("SELECT count(*) FROM comic_chapter WHERE comic_id = $1 AND c_order = $2", c.Comic.ID, c.Order).Scan(&count); err != nil {
		log.Println(err)
		return err
	}

	if count > 0 {
		return errors.New("A chapter with that order already exists")
	}

	err := q.QueryRow("INSERT INTO comic_Chapter(comic_id, c_order, title) VALUES($1, $2, $3) RETURNING id", c.Comic.QID(q), c.Order, c.QTitle(q)).Scan(&c.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	if err = c.log(q, lCreate, user); err != nil {
		log.Println(err)
	}

	C.Cache.Purge(cacheComic, strconv.Itoa(c.Comic.ID))

	return err
}

func (c *Chapter) SaveEdit(q querier.Q, user *User) error {
	if c.QID(q) <= 0 {
		return errors.New("Chapter doesn't exist")
	}

	if c.Comic.QID(q) <= 0 {
		return errors.New("Comic doesn't exist")
	}

	var i int

	if err := q.QueryRow("SELECT id FROM comic_chapter WHERE comic_id = $1 AND c_order = $2", c.Comic.ID, c.Order).Scan(&i); err != nil {
		if err != sql.ErrNoRows {
			log.Println(err)
			return err
		}
	}

	if i > 0 && i != c.ID {
		return errors.New("A chapter with that order already exists")
	}

	_, err := q.Exec(
		"UPDATE comic_chapter SET comic_id = $1, c_order = $2, title = $3 WHERE id = $4",
		c.Comic.ID,
		c.Order,
		c.Title,
		c.ID,
	)
	if err != nil {
		log.Println(err)
		return err
	}

	if err = c.log(q, lUpdate, user); err != nil {
		log.Println(err)
	}

	C.Cache.Purge(cacheComic, strconv.Itoa(c.Comic.ID))

	return err
}

func (c *Chapter) Delete(user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	var a func() error
	a = tx.Rollback
	defer func() { a() }()

	if err = c.log(tx, lRemove, user); err != nil {
		log.Println(err)
		return err
	}

	_, err = tx.Exec("DELETE FROM comic_chapter WHERE id = $1", c.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	a = tx.Commit

	err = c.QComic(tx)
	if err != nil {
		return err
	}

	C.Cache.Purge(cacheComic, strconv.Itoa(c.Comic.ID))

	return nil
}

func (c *Chapter) QPosts(q querier.Q) []*ComicPost {
	if c.QID(q) == 0 {
		return nil
	}
	if len(c.Posts) > 0 {
		return c.Posts
	}

	str := "SELECT id, post_id, post_order FROM comic_mappings WHERE Chapter_id=$1 ORDER BY post_order"
	rows, err := q.Query(str, c.QID(q))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer rows.Close()

	//var cps []*ComicPost

	for rows.Next() {
		cp := newComicPost()
		cp.Chapter = c
		err = rows.Scan(&cp.ID, &cp.Post.ID, &cp.Order)
		if err != nil {
			log.Print(err)
			return nil
		}
		c.Posts = append(c.Posts, cp)
	}
	err = rows.Err()
	if err != nil {
		log.Print(err)
		return nil
	}

	return c.Posts
}

func (c *Chapter) PostsLimit(limit int) []*ComicPost {
	return c.QPosts(DB)[:max(limit, len(c.QPosts(DB)))]
}

func (c *Chapter) NewComicPost() *ComicPost {
	cp := newComicPost()
	cp.Comic = c.Comic
	cp.Chapter = c
	return cp
}
