package DataManager

import (
	"database/sql"
	"errors"
	"log"
)

func NewChapter() *Chapter {
	return &Chapter{}
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

func (c *Chapter) QID(q querier) int {
	if c.ID != 0 {
		return c.ID
	}
	if c.Comic.QID(q) == 0 {
		return 0
	}
	if c.Order == 0 {
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

func (c *Chapter) QComic(q querier) error {
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

func (c *Chapter) QTitle(q querier) string {
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

func (c *Chapter) QOrder(q querier) int {
	if c.Order != 0 {
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

func (c *Chapter) QPageCount(q querier) int {
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

func (c *Chapter) Save(q querier, user *User) error {
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

	return err
}

func (c *Chapter) SaveEdit(q querier, user *User) error {
	if c.QID(q) <= 0 {
		return errors.New("Chapter doesn't exist")
	}

	if c.Comic.QID(q) <= 0 {
		return errors.New("Comic doesn't exist")
	}

	var count int

	if err := q.QueryRow("SELECT count(*) FROM comic_chapter WHERE comic_id = $1 AND c_order = $2", c.Comic.ID, c.Order).Scan(&count); err != nil {
		log.Println(err)
		return err
	}

	if count > 0 {
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

	if err = c.log(q, lUpdate, user); err != nil{
		log.Println(err)
	}

	return err
}

func (c *Chapter) QPosts(q querier) []*ComicPost {
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
