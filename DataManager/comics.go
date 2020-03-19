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



func (cc *ComicCollector) Search(tagQuery string, limit, offset int) error {
	tags, err := parseTags(tagQuery)
	if err != nil || tags == nil {
		return err
	}

	ptmJoin := fmt.Sprintf(`
		JOIN post_tag_mappings ptm0
		ON cm.post_id = ptm0.post_id
		%s`,
		ptmJoinQuery(tags),
	)

	ptmWhere := ptmWhereQuery(tags)

	meat := fmt.Sprintf(`
		FROM comics c
		JOIN comic_chapter cc
		ON c.id = cc.comic_id
		JOIN comic_mappings cm
		ON cc.id = cm.chapter_id
		%s
		WHERE %s
		`,
		ptmJoin,
		ptmWhere,
	)

	query := fmt.Sprintf(`
		SELECT DISTINCT c.id, c.title, c.modified
		%s
		ORDER BY c.modified DESC
		LIMIT $1
		OFFSET $2
		`,
		meat,
	)

	queryFn := func() error {
		rows, err := DB.Query(query, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var comic = NewComic()
			var garbage string
			err = rows.Scan(&comic.ID, &comic.Title, &garbage)
			if err != nil {
				return err
			}

			cc.Comics = append(cc.Comics, comic)
		}

		return rows.Err()
	}

	if err = queryFn(); err != nil {
		return err
	}

	query = fmt.Sprintf(`
		SELECT count(DISTINCT c.id)
		%s
		`,
		meat,
	)

	err = DB.QueryRow(query).Scan(&cc.TotalComics)

	return err
}

func parseTags(tagQuery string) ([]*Tag, error) {
	var tc TagCollector

	err := tc.Parse(tagQuery)
	if err != nil {
		return nil, err
	}

	var tags []*Tag
	for _, tag := range tc.Tags {
		if tag.QID(DB) == 0 {
			// No posts will be available, return
			return nil, nil
		}

		alias := NewAlias()
		alias.Tag = tag
		if alias.QTo(DB).QID(DB) != 0 {
			tag = alias.QTo(DB)
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func ptmWhereQuery(tags []*Tag) string {
	var ptmWhere string

	for i := 0; i < len(tags) - 1; i++ {
		ptmWhere += fmt.Sprintf(
			"ptm%d.tag_id = %d AND ",
			i,
			tags[i].ID,
		)
	}

	ptmWhere += fmt.Sprintf(
		"ptm%d.tag_id = %d",
		len(tags)-1,
		tags[len(tags)-1].ID,
	)

	return ptmWhere
}

func ptmJoinQuery(tags []*Tag) string {
	if len(tags) < 2 {
		return ""
	}

	var ptmJoin string
	for i := 0; i < len(tags) - 1; i++{
		ptmJoin += fmt.Sprintf(`
			JOIN post_tag_mappings ptm%d
			ON ptm%d.post_id = ptm%d.post_id
			`,
			i+1,
			i,
			i+1,
		)
	}

	return ptmJoin
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

func (c *Comic) Save(user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	var a func() error
	a = tx.Rollback

	defer func() {
		a()
	}()

	if c.QID(tx) != 0 {
		return errors.New("Comic already exist")
	}

	if c.QTitle(tx) == "" {
		return errors.New("Title is empty")
	}

	err = tx.QueryRow("INSERT INTO comics(title) VALUES($1) RETURNING id", c.QTitle(tx)).Scan(&c.ID)
	if err != nil {
		log.Print(err)
		return err
	}

	err = c.log(tx, lCreate, user)
	if err != nil {
		log.Println(err)
	}

	a = tx.Commit

	return err
}

func (c *Comic) SaveEdit(user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	var a func() error
	a = tx.Rollback

	defer func() {
		a()
	}()

	if c.QID(tx) <= 0 {
		return errors.New(fmt.Sprint("Invalid comic id: ", c.ID))
	}

	err = c.log(tx, lUpdate, user)
	if err != nil {
		log.Println(err)
		return err
	}

	_, err = tx.Exec("UPDATE comics SET title = $1 WHERE id = $2", c.Title, c.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	a = tx.Commit

	return nil
}

func (c *Comic) Delete(user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	var a func() error
	a = tx.Rollback

	defer func() {
		a()
	}()

	if c.QID(tx) <= 0 {
		return errors.New(fmt.Sprint("Invalid comic id: ", c.ID))
	}

	if err = c.log(tx, lRemove, user); err != nil {
		log.Println(err)
		return err
	}

	_, err = tx.Exec("DELETE FROM comics WHERE id = $1", c.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	a = tx.Commit

	return nil
}
