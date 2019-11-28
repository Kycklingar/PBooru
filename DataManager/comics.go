package DataManager

import (
	"errors"
	"fmt"
	"log"
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
	err = DB.QueryRow("SELECT count(*) FROM comics").Scan(&cc.TotalComics)
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func NewComic() *Comic {
	return &Comic{}
}

func NewComicByID(id int) (*Comic, error) {
	comic := new(Comic)
	if err := DB.QueryRow("SELECT id FROM comics WHERE id = $1", id).Scan(&comic.ID); err != nil {
		return nil, err
	}

	return comic, nil
}

type Comic struct {
	ID           int
	Title        string
	Posts        []*ComicPost
	Chapters     []*Chapter
	ChapterCount int
	PageCount    int
}

func (c *Comic) QID(q querier) int {
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

func (c *Comic) Chapter(q querier, order int) *Chapter {
	ch := newChapter()
	ch.Comic = c
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

	rows, err := q.Query("SELECT id, c_order, title FROM comic_chapter WHERE comic_id=$1 ORDER BY c_order", c.QID(q))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		ch := newChapter()
		ch.Comic = c

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

	err := q.QueryRow("SELECT count(*) FROM comic_mappings WHERE chapter_id IN(SELECT id FROM comic_chapter WHERE comic_id = $1)", c.QID(q)).Scan(&c.PageCount)
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

	err := q.QueryRow("SELECT count(*) FROM comic_Chapter WHERE comic_id=$1", c.QID(q)).Scan(&c.ChapterCount)
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
