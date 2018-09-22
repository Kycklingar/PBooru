package DataManager

import (
	"fmt"
	"log"
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

		err = rows.Scan(&th.id, &th.User.ID, &th.Post.ID, &th.Timestamp)
		if err != nil {
			log.Print(err)
			return nil
		}
		thr = append(thr, th)
	}
	return thr
}

func NewTagHistory() *TagHistory {
	return &TagHistory{User: NewUser(), Post: NewPost()}
}

type TagHistory struct {
	id int

	User      *User
	Post      *Post
	Timestamp string

	ETags []*EditedTag
}

func (th *TagHistory) ID(q querier) int {
	if th.id != 0 {
		return th.id
	}

	if th.User.QID(q) == 0 || th.Post.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT id FROM tag_history WHERE user_id=? AND post_id=?", th.User.QID(q), th.Post.QID(q)).Scan(&th.id)
	if err != nil {
		log.Print(err)
		return 0
	}
	return th.id
}

func (th *TagHistory) QETags(q querier) []*EditedTag {
	if th.ETags != nil {
		return th.ETags
	}

	if th.ID(q) == 0 {
		return nil
	}

	rows, err := q.Query("SELECT tag_id, direction FROM edited_tags WHERE history_id=?", th.ID(q))
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
