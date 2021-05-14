package DataManager

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

const (
	cacheComic = "CMC"
)

type ComicCollector struct {
	Comics      []*Comic
	TotalComics int
}

func (cc *ComicCollector) Search(title, tagQuery string, limit, offset int) error {
	tags, err := parseTags(tagQuery)
	if err != nil {
		return err
	}

	ptmJoin := fmt.Sprintf(`
		JOIN post_tag_mappings ptm0
		ON cm.post_id = ptm0.post_id
		%s`,
		ptmJoinQuery(tags),
	)

	where := ptmWhereQuery(tags)

	var parN = 1
	if len(title) > 0 {
		if where != "" {
			where += `
				AND `
		}
		where += `
			(lower(c.title) like '%' || $1 || '%'
			OR lower(cc.title) like '%' || $1 || '%')
		`

		parN++
	}

	var meat string

	if where != "" {
		where = "WHERE " + where
		meat = fmt.Sprintf(`
			FROM comics c
			JOIN comic_chapter cc
			ON c.id = cc.comic_id
			JOIN comic_mappings cm
			ON cc.id = cm.chapter_id
			%s
			%s
			`,
			ptmJoin,
			where,
		)
	} else {
		meat = `
			FROM comics c
		`
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT c.id, c.title, c.modified
		%s
		ORDER BY c.modified DESC
		LIMIT $%d
		OFFSET $%d
		`,
		meat,
		parN,
		parN+1,
	)

	params := func(additional ...interface{}) []interface{} {
		if title != "" {
			return append([]interface{}{title}, additional...)
		}
		return additional
	}

	queryFn := func() error {
		rows, err := DB.Query(query, params(limit, offset)...)
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

			comic = CachedComic(comic)

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

	err = DB.QueryRow(query, params()...).Scan(&cc.TotalComics)

	return err
}

func parseTags(tagQuery string) ([]*Tag, error) {
	if len(tagQuery) <= 0 {
		return nil, nil
	}
	var tc TagCollector

	err := tc.Parse(tagQuery, ",")
	if err != nil {
		return nil, err
	}

	tc.upgrade(DB, false)

	return tc.Tags, nil
}

func ptmWhereQuery(tags []*Tag) string {
	if len(tags) <= 0 {
		return ""
	}

	var ptmWhere string

	for i := 0; i < len(tags)-1; i++ {
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
	for i := 0; i < len(tags)-1; i++ {
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

func CachedComic(c *Comic) *Comic {
	if cc := C.Cache.Get(cacheComic, strconv.Itoa(c.ID)); cc != nil {
		return cc.(*Comic)
	}

	C.Cache.Set(cacheComic, strconv.Itoa(c.ID), c)
	return c
}

type Comic struct {
	ID           int
	Title        string
	Posts        []*ComicPost
	Chapters     []*Chapter
	ChapterCount int
	PageCount    int

	TagSummary []*Tag
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

func (c *Comic) QTagSummary(q querier) error {
	if len(c.TagSummary) > 0 {
		return nil
	}

	rows, err := q.Query(`
		SELECT ptm.tag_id
		FROM post_tag_mappings ptm
		JOIN comic_mappings cm
			ON ptm.post_id = cm.post_id
		JOIN comic_chapter cc
			ON cm.chapter_id = cc.id
		JOIN comics c
			ON cc.comic_id = c.id
		WHERE c.id = $1
		GROUP BY ptm.tag_id ORDER BY count(ptm.tag_id) DESC LIMIT 15;
		`,
		c.ID,
	)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t = NewTag()
		err = rows.Scan(&t.ID)
		if err != nil {
			return err
		}

		t = CachedTag(t)
		t.QTag(q)
		t.QNamespace(q)
		t.Namespace.QNamespace(q)

		c.TagSummary = append(c.TagSummary, t)
	}

	return rows.Err()
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

	C.Cache.Purge(cacheComic, strconv.Itoa(c.ID))

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

	C.Cache.Purge(cacheComic, strconv.Itoa(c.ID))

	return nil
}
