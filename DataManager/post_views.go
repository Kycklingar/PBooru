package DataManager

import (
	"log"
	"sync"
	"time"
)

func RegisterPostView(postID int) {
	postViewer.reg(postID)
}

const collectionPeriod = time.Hour

var postViewer postViews

type postViews struct {
	l     sync.Mutex
	views map[int]int
}

func (pv *postViews) reg(postID int) {
	pv.l.Lock()
	defer pv.l.Unlock()

	if _, ok := pv.views[postID]; !ok {
		pv.views[postID] = 0
	}

	pv.views[postID]++
}

func (pv *postViews) collect() {
	defer time.AfterFunc(collectionPeriod, pv.collect)

	pv.l.Lock()
	views := pv.views
	pv.views = make(map[int]int)
	pv.l.Unlock()

	func() {
		stmt, err := DB.Prepare(`
			INSERT INTO post_views (
				post_id,
				views
			)
			VALUES(
				$1,
				$2
			)
			`,
		)
		if err != nil {
			log.Println(err)
			return
		}
		defer stmt.Close()

		for postID, viewcount := range views {
			stmt.Exec(postID, viewcount)
		}
	}()

	func() {
		stmt, err := DB.Prepare(`
			UPDATE posts
			SET score = (
				SELECT count(*)
				FROM post_score_mapping
				WHERE post_id = $1
			) + (
				SELECT COALESCE(SUM(views), 0) / 1000
				FROM post_views
				WHERE post_id = $1
			)
			WHERE id = $1
			`,
		)
		if err != nil {
			log.Println(err)
			return
		}
		defer stmt.Close()

		for postID, _ := range views {
			stmt.Exec(postID)
		}
	}()
}

func init() {
	postViewer.views = make(map[int]int)
	time.AfterFunc(collectionPeriod, postViewer.collect)
}
