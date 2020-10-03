package DataManager

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

func GetTagHistory(limit, offset int) []*TagHistory {
	rows, err := DB.Query(fmt.Sprintf("SELECT id, user_id, post_id, timestamp FROM tag_history ORDER BY timestamp DESC LIMIT %d OFFSET %d", limit, offset))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer rows.Close()

	var thr []*TagHistory

	for rows.Next() {
		th := NewTagHistory()

		err = rows.Scan(&th.ID, &th.User.ID, &th.Post.ID, &th.Timestamp)
		if err != nil {
			log.Print(err)
			return nil
		}
		thr = append(thr, th)
	}
	return thr
}

func GetUserTagHistory(limit, offset, userID int) ([]*TagHistory, int) {
	var total int
	err := DB.QueryRow("SELECT count(*) FROM tag_history WHERE user_id = $1", userID).Scan(&total)
	if err != nil {
		return nil, 0
	}
	rows, err := DB.Query(fmt.Sprintf("SELECT id, user_id, post_id, timestamp FROM tag_history WHERE user_id = $1 ORDER BY timestamp DESC LIMIT %d OFFSET %d", limit, offset), userID)
	if err != nil {
		log.Print(err)
		return nil, 0
	}
	defer rows.Close()

	var thr []*TagHistory

	for rows.Next() {
		th := NewTagHistory()

		err = rows.Scan(&th.ID, &th.User.ID, &th.Post.ID, &th.Timestamp)
		if err != nil {
			log.Print(err)
			return nil, 0
		}
		thr = append(thr, th)
	}
	return thr, total
}

func NewTagHistory() *TagHistory {
	return &TagHistory{User: NewUser(), Post: NewPost()}
}

type TagHistory struct {
	ID int

	User      *User
	Post      *Post
	Timestamp timestamp

	ETags []*EditedTag
}

func (th *TagHistory) Reverse() error {
	if th.ID <= 0 {
		return errors.New("no taghistory id to reverse")
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}

	etags := th.QETags(tx)
	th.QPost(tx)

	if th.Post.ID <= 0 {
		return txError(tx, errors.New("taghistory post has no id"))
	}

	for _, et := range etags {
		if et.Tag.QID(tx) == 0 {
			return txError(tx, errors.New("tag has no id!"))
		}

		if et.Direction {
			_, err = tx.Exec("DELETE FROM post_tag_mappings WHERE post_id = $1 AND tag_id = $2", th.Post.ID, et.Tag.QID(tx))
		} else {
			_, err = tx.Exec("INSERT INTO post_tag_mappings (post_id, tag_id) VALUES($1, $2) ON CONFLICT DO NOTHING", th.Post.ID, et.Tag.QID(tx))
		}

		if err != nil {
			log.Println(err)
			return txError(tx, err)
		}
	}

	if _, err = tx.Exec("DELETE FROM edited_tags WHERE history_id = $1", th.ID); err != nil {
		log.Println(err)
		return txError(tx, err)
	}

	if _, err = tx.Exec("DELETE FROM tag_history WHERE id = $1", th.ID); err != nil {
		log.Println(err)
		return txError(tx, err)
	}

	if err = tx.Commit(); err != nil {
		log.Println(err)
		return err
	}

	for _, et := range etags {
		resetCacheTag(tx, et.Tag.QID(DB))
	}

	// Reset post cache
	C.Cache.Purge("TC", strconv.Itoa(th.Post.ID))

	return nil
}

func (th *TagHistory) QPost(q querier) error {
	if th.ID <= 0 {
		return errors.New("taghistory has no ID")
	}

	return q.QueryRow("SELECT post_id FROM tag_history WHERE id = $1", th.ID).Scan(&th.Post.ID)
}

func (th *TagHistory) QUser(q querier) error {
	if th.ID <= 0 {
		return errors.New("taghistory has no ID")
	}

	return q.QueryRow("SELECT user_id FROM tag_history WHERE id = $1", th.ID).Scan(&th.User.ID)
}

func (th *TagHistory) QETags(q querier) []*EditedTag {
	if th.ETags != nil {
		return th.ETags
	}

	if th.ID == 0 {
		return nil
	}

	rows, err := q.Query("SELECT tag_id, direction FROM edited_tags WHERE history_id=$1", th.ID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		et := newEditedTag()
		//et.setQ(th.Q(q))
		var dir int
		err = rows.Scan(&et.Tag.ID, &dir)
		if err != nil {
			log.Print(err)
			return nil
		}

		if dir == 1 {
			// Tag was added
			et.Direction = true
		} else if dir == -1 {
			// Tag was removed
			et.Direction = false
		} else {
			log.Print("Direction was incorrect")
			return nil
		}
		th.ETags = append(th.ETags, et)
	}
	if rows.Err() != nil {
		log.Print(rows.Err())
		return nil
	}
	return th.ETags
}

func newEditedTag() *EditedTag {
	return &EditedTag{Tag: NewTag(), Direction: false}
}

type EditedTag struct {
	Tag       *Tag
	Direction bool
}
