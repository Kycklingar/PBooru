package DataManager

type logAction int

const (
	lCreate logAction = iota
	lUpdate
	lRemove
)

func (c *Comic) log(q querier, action logAction, user *User) error {
	_, err := q.Exec(`
		INSERT INTO log_comic(
			action,
			comic_id,
			user_id,
			title
			)
		VALUES($1, $2, $3, $4)`,
		action,
		c.ID,
		user.ID,
		c.Title,
	)
	return err
}

func (ch *Chapter) log(q querier, action logAction, user *User) error {
	_, err := q.Exec(`
		INSERT INTO log_chapter(
			action,
			chapter_id,
			user_id,
			comic_id,
			c_order,
			title
			)
		VALUES($1, $2, $3, $4, $5, $6)`,
		action,
		ch.ID,
		user.ID,
		ch.Comic.ID,
		ch.Order,
		ch.Title,
	)
	return err
}

func (cp *ComicPost) log(q querier, action logAction, user *User) error {
	_, err := q.Exec(`
		INSERT INTO log_comic_page(
			action,
			comic_page_id,
			user_id,
			post_id,
			chapter_id,
			page
			)
		VALUES($1, $2, $3, $4, $5, $6)`,
		action,
		cp.ID,
		user.ID,
		cp.Post.ID,
		cp.Chapter.ID,
		cp.Order,
	)
	return err
}
