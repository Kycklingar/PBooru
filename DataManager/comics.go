package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	C "github.com/kycklingar/PBooru/DataManager/cache"
	"log"
	"strconv"
)

type ComicCollector struct {
	Comics      []*Comic
	TotalComics int
}

func (cc *ComicCollector) Get(limit, offset int) error {
	str := fmt.Sprintf("SELECT id, title FROM comics ORDER BY modified DESC LIMIT %d OFFSET %d", limit, offset)
	rows, err := DB.Query(str)
	if err != nil {
		log.Print(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		c := NewComic()
		err = rows.Scan(&c.ID, &c.Title)
		if err != nil {
			log.Print(err)
			return err
		}
		cc.Comics = append(cc.Comics, c)
	}
	if rows.Err() != nil {
		log.Print(rows.Err())
		return err
	}
	err = DB.QueryRow("SELECT count() FROM comics").Scan(&cc.TotalComics)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func NewComic() *Comic {
	return &Comic{}
}

type Comic struct {
	ID           int
	Title        string
	Posts        []*ComicPost
	Chapters     []*Chapter
	ChapterCount int
	PageCount    int
	//q         querier
}

func (c *Comic) QID(q querier) int {
	//return c.ID
	if c.ID != 0 {
		return c.ID
	}
	if c.Title == "" {
		return 0
	}
	err := q.QueryRow("SELECT id FROM comics WHERE title=$1", c.QTitle(q)).Scan(&c.ID)
	if err != nil {
		return 0
	}
	return c.ID
}

func (c *Comic) SetID(id int) error {
	c.ID = id
	return nil
	//err := q.QueryRow("SELECT id FROM comics WHERE id=$1", id).Scan(&c.ID)
	//return err
}

func (c *Comic) QTitle(q querier) string {
	if c.Title != "" {
		return c.Title
	}
	if c.QID(q) == 0 {
		return ""
	}

	err := q.QueryRow("SELECT title FROM comics WHERE id=$1", c.ID).Scan(&c.Title)
	if err != nil {
		log.Print(err)
		return ""
	}
	return c.Title
}

func (c *Comic) SetTitle(title string) {
	c.Title = title
}

func (c *Comic) postsQuery(q querier, str string, ch *Chapter) []*ComicPost {
	var rows *sql.Rows
	var err error
	if ch.QID(q) != 0 {
		rows, err = q.Query(str, c.QID(q), ch.ID)
	} else {
		rows, err = q.Query(str, c.QID(q))
	}
	if err != nil {
		log.Print(err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		cp := newComicPost()
		cp.Chapter = ch

		err = rows.Scan(&cp.Post.ID, &cp.Order)
		if err != nil {
			log.Print(err)
			return nil
		}
		c.Posts = append(c.Posts, cp)
	}
	if rows.Err() != nil {
		log.Print(rows.Err())
		return nil
	}
	return c.Posts
}

func (c *Comic) QPosts(q querier) []*ComicPost {
	if c.QID(q) == 0 {
		return nil
	}
	if len(c.Posts) > 0 {
		return c.Posts
	}

	str := fmt.Sprintf("SELECT post_id, post_order FROM comic_mappings WHERE comic_id=$1 ORDER BY post_order")

	return c.postsQuery(q, str, newChapter())
}

func (c *Comic) PostsCh(q querier, ch *Chapter) []*ComicPost {
	if c.QID(q) == 0 {
		return nil
	}
	if len(c.Posts) > 0 {
		return c.Posts
	}
	var str string
	if ch.QID(q) == 0 {
		str = fmt.Sprintf("SELECT post_id, post_order FROM comic_mappings WHERE comic_id=$1 ORDER BY post_order")
	} else {
		str = fmt.Sprintf("SELECT post_id, post_order FROM comic_mappings WHERE comic_id=$1 AND Chapter_id=$2 ORDER BY post_order")
	}

	return c.postsQuery(q, str, ch)
}

func (c *Comic) NewChapter() *Chapter {
	ch := newChapter()
	ch.Comic = c
	return ch
}

func (c *Comic) Chapter(q querier, order int) *Chapter {
	ch := c.NewChapter()
	ch.Order = order

	if ch.QID(q) == 0 {
		return nil
	}

	return ch
}

func (c *Comic) QChapters(q querier) []*Chapter {
	if len(c.Chapters) > 0 {
		return c.Chapters
	}

	if c.QID(q) == 0 {
		return nil
	}

	rows, err := q.Query("SELECT id, c_order, title FROM comic_Chapter WHERE comic_id=$1 ORDER BY c_order", c.QID(q))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		ch := c.NewChapter()

		err = rows.Scan(&ch.ID, &ch.Order, &ch.Title)
		if err != nil {
			log.Print(err)
			return nil
		}
		c.Chapters = append(c.Chapters, ch)
	}
	if rows.Err() != nil {
		log.Print(err)
		return nil
	}
	return c.Chapters
}

func (c *Comic) ChaptersLimit(limit int) []*Chapter {
	return c.QChapters(DB)[:max(limit, c.QChapterCount(DB))]
}

func (c *Comic) QPageCount(q querier) int {
	if c.PageCount != 0 {
		return c.PageCount
	}
	if c.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT count() FROM comic_mappings WHERE comic_id=$1", c.QID(q)).Scan(&c.PageCount)
	if err != nil {
		log.Print(err)
		return 0
	}
	return c.PageCount
}

func (c *Comic) QChapterCount(q querier) int {
	if c.ChapterCount != 0 {
		return c.ChapterCount
	}
	if c.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT count() FROM comic_Chapter WHERE comic_id=$1", c.QID(q)).Scan(&c.ChapterCount)
	if err != nil {
		log.Print(err)
		return 0
	}
	return c.ChapterCount
}

func (c *Comic) Save(q querier) error {
	if c.QID(q) != 0 {
		return errors.New("Comic already exist")
	}
	if c.QTitle(q) == "" {
		return errors.New("Title is empty")
	}
	st, err := q.Exec("INSERT INTO comics(title) VALUES($1)", c.QTitle(q))
	if err != nil {
		log.Print(err)
		return err
	}
	id64, err := st.LastInsertId()
	if err != nil {
		log.Print(err)
		return err
	}

	c.ID = int(id64)

	return err
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

	if err := q.QueryRow("SELECT count() FROM comic_mappings WHERE chapter_id = $1", c.QID(q)).Scan(&c.PageCount); err != nil {
		log.Println(err)
		return 0
	}

	return c.PageCount
}

func (c *Chapter) Save(q querier) error {
	if c.QID(q) != 0 {
		return errors.New("Chapter already exist")
	}

	if c.Comic.QID(q) == 0 {
		return errors.New("comic id not set")
	}
	// if c.Title() == "" {
	// 	return errors.New("title is empty")
	// }
	if c.QOrder(q) == 0 {
		return errors.New("order is 0")
	}

	_, err := q.Exec("INSERT INTO comic_Chapter(comic_id, c_order, title) VALUES($1, $2, $3)", c.Comic.QID(q), c.Order, c.QTitle(q))

	C.Cache.Purge("CCH", strconv.Itoa(c.QID(q)))

	return err
}

func (c *Chapter) QPosts(q querier) []*ComicPost {
	if c.QID(q) == 0 {
		return nil
	}
	if len(c.Posts) > 0 {
		return c.Posts
	}

	if m := C.Cache.Get("CCH", strconv.Itoa(c.QID(q))); m != nil {
		switch mm := m.(type) {
		case *Chapter:
			*c = *mm
			return c.Posts
		}
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

	C.Cache.Set("CCH", strconv.Itoa(c.QID(q)), c)
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

func newComicPost() *ComicPost {
	var cp ComicPost
	cp.Chapter = newChapter()
	cp.Post = NewPost()
	cp.Comic = NewComic()
	cp.Order = 0
	return &cp
}

type ComicPost struct {
	ID      int
	Post    *Post
	Order   int
	Chapter *Chapter
	Comic   *Comic
}

func (p *ComicPost) QID(q querier) int {
	if p.ID != 0 {
		return p.ID
	}

	err := q.QueryRow("SELECT id FROM comic_mappings WHERE comic_id=$1 AND post_id=$2", p.Comic.QID(q), p.Post.QID(q)).Scan(&p.ID)
	if err != nil {
		log.Print(err)
	}
	return p.ID
}

func (p *ComicPost) QOrder(q querier) int {
	return p.Order
}

func (p *ComicPost) Save(q querier, overwrite bool) error {
	if p.Post.QID(q) == 0 {
		return fmt.Errorf("Invalid post")
	}
	if p.Comic.QID(q) == 0 {
		return fmt.Errorf("Invalid comic")
	}
	if p.Chapter.QID(q) == 0 {
		return fmt.Errorf("Invalid Chapter")
	}

	if overwrite && p.QID(q) != 0 {
		_, err := q.Exec("UPDATE comic_mappings SET post_order=$1, Chapter_id=$2 WHERE comic_id=$3 AND post_id=$4", p.Order, p.Chapter.QID(q), p.Comic.QID(q), p.Post.QID(q))
		if err != nil {
			log.Print(err)
			return err
		}
	} else {
		_, err := q.Exec("INSERT INTO comic_mappings(comic_id, post_id, post_order, Chapter_id) Values($1, $2, $3, $4)", p.Comic.QID(q), p.Post.QID(q), p.Order, p.Chapter.QID(q))
		if err != nil {
			log.Print(err)
			return err
		}
	}

	C.Cache.Purge("CCH", strconv.Itoa(p.Chapter.QID(q)))

	return nil
}

func (p *ComicPost) replacePost(q querier, new *Post) error {
	if new.QID(q) == 0 {
		return errors.New("new.id is zero")
	}
	if p.Post.QID(q) == 0 {
		return errors.New("p.ID is zero")
	}
	_, err := q.Exec("UPDATE comic_mappings SET post_id=$1 WHERE post_id=$2", new.QID(q), p.Post.QID(q))
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}
