package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
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
	err = DB.QueryRow("SELECT count(1) FROM comics").Scan(&cc.TotalComics)
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

	err := q.QueryRow("SELECT count(1) FROM comic_mappings WHERE comic_id=$1", c.QID(q)).Scan(&c.PageCount)
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

	err := q.QueryRow("SELECT count(1) FROM comic_Chapter WHERE comic_id=$1", c.QID(q)).Scan(&c.ChapterCount)
	if err != nil {
		log.Print(err)
		return 0
	}
	return c.ChapterCount
}

func (c *Comic) Save(q querier, user *User) error {
	if c.QID(q) != 0 {
		return errors.New("Comic already exist")
	}

	if c.QTitle(q) == "" {
		return errors.New("Title is empty")
	}

	err := q.QueryRow("INSERT INTO comics(title) VALUES($1) RETURNING id", c.QTitle(q)).Scan(&c.ID)
	if err != nil {
		log.Print(err)
		return err
	}

	err = c.log(q, lCreate, user)
	if err != nil {
		log.Println(err)
	}

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

	if err := q.QueryRow("SELECT count(1) FROM comic_mappings WHERE chapter_id = $1", c.QID(q)).Scan(&c.PageCount); err != nil {
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
	// if c.Title() == "" {
	// 	return errors.New("title is empty")
	// }
	if c.QOrder(q) == 0 {
		return errors.New("order is 0")
	}

	err := q.QueryRow("INSERT INTO comic_Chapter(comic_id, c_order, title) VALUES($1, $2, $3) RETURNING id", c.Comic.QID(q), c.Order, c.QTitle(q)).Scan(&c.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	if err = c.log(q, lCreate, user); err != nil {
		log.Println(err)
	}

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
func NewComicPost() *ComicPost {
	var cp ComicPost
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

	err := q.QueryRow("SELECT id FROM comic_mappings WHERE comic_id=$1 AND chapter_id = $2 AND post_id=$3", p.Comic.QID(q), p.Chapter.QID(q), p.Post.QID(q)).Scan(&p.ID)
	if err != nil {
		log.Print(err)
	}
	return p.ID
}

func (p *ComicPost) QChapter(q querier) error {
	if p.Chapter != nil {
		return nil
	}

	var chapter Chapter
	if err := q.QueryRow("SELECT chapter_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&chapter.ID); err != nil {
		log.Println(err)
		return err
	}

	p.Chapter = &chapter

	return nil
}

func (p *ComicPost) QComic(q querier) error {
	if p.Comic != nil {
		return nil
	}

	comic := NewComic()
	if err := q.QueryRow("SELECT comic_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&comic.ID); err != nil {
		log.Println(err)
		return err
	}

	p.Comic = comic

	return nil
}

func (p *ComicPost) QPost(q querier) error {
	if p.Post != nil {
		return nil
	}

	var post = NewPost()

	if err := q.QueryRow("SELECT post_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&post.ID); err != nil {
		log.Println(err)
		return nil
	}

	p.Post = post

	return nil
}

func (p *ComicPost) QOrder(q querier) int {
	if p.Order >= 0 {
		return p.Order
	}
	if err := q.QueryRow("SELECT post_order FROM comic_mappings WHERE id = $1", p.ID).Scan(&p.Order); err != nil {
		log.Println(err)
	}
	return p.Order
}

func (p *ComicPost) Save(user *User, overwrite bool) error {
	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}

	defer tx.Commit()

	if !user.QFlag(tx).Comics() {
		return fmt.Errorf("Action not allowed from this user")
	}

	if p.Chapter.QID(tx) == 0 {
		return fmt.Errorf("Invalid Chapter")
	}



	if overwrite && p.QID(tx) != 0 {
		_, err := tx.Exec("UPDATE comic_mappings SET post_order=$1, Chapter_id=$2 WHERE id = $3", p.Order, p.Chapter.QID(tx), p.ID)
		if err != nil {
			log.Print(err)
			tx.Rollback()
			return err
		}

		p.QComic(tx)
		p.Comic.QID(tx)
		p.QChapter(tx)
		p.Chapter.QID(tx)
		p.QPost(tx)
		p.Post.QID(tx)

		if err = p.log(tx, lUpdate, user); err != nil {
			log.Println(err)
			tx.Rollback()
			return err
		}

	} else {
		if p.Post.QID(tx) == 0 {
			return fmt.Errorf("Invalid post")
		}

		if p.Comic.QID(tx) == 0 {
			return fmt.Errorf("Invalid comic")
		}

		err := tx.QueryRow("INSERT INTO comic_mappings(comic_id, post_id, post_order, Chapter_id) Values($1, $2, $3, $4) RETURNING id", p.Comic.QID(tx), p.Post.QID(tx), p.Order, p.Chapter.QID(tx)).Scan(&p.ID)
		if err != nil {
			log.Print(err)
			tx.Rollback()
			return err
		}

		if err = p.log(tx, lCreate, user); err != nil {
			log.Println(err)
			tx.Rollback()
			return err
		}
	}

	C.Cache.Purge("CCH", strconv.Itoa(p.Chapter.QID(tx)))

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
