package DataManager

import (
	"database/sql"
)

const (
	lPostDescription   logtable = "post_descr"
	lPostCreationDates logtable = "post_creation"
	lPostMetaData      logtable = "post_metadata"
	lPostTags          logtable = "post_tags"
)

func init() {
	logTableGetFuncs[lPostDescription] = getLogPostDescriptions
	logTableGetFuncs[lPostCreationDates] = getLogPostCreationDates
	logTableGetFuncs[lPostMetaData] = getLogPostMetaData
	logTableGetFuncs[lPostTags] = getLogPostTags
}

type postHistoryMap map[int]postHistory

type postHistory struct {
	Post          *Post
	Description   logPostDescription
	MetaData      []logPostMetaData
	CreationDates []logPostCreationDates
	Tags          logPostTags
}

func (l *Log) initPostHistory(postID int) postHistory {
	p := l.Posts[postID]
	if p.Post == nil {
		p.Post = NewPost()
	}
	p.Post.ID = postID

	return p
}

func initPostLog(tx *sql.Tx, logID, postID int) error {
	_, err := tx.Exec(`
		INSERT INTO log_post(
			log_id,
			post_id
		)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		`,
		logID,
		postID,
	)

	return err
}

func logAffectedPosts(logID int, tx *sql.Tx, pids []int) error {
	stmt, err := tx.Prepare(`
		INSERT INTO log_post(
			log_id,
			post_id
		)
		VALUES($1, $2)
		ON CONFLICT DO NOTHING
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range pids {
		if _, err = stmt.Exec(logID, p); err != nil {
			return err
		}
	}

	return nil
}

func getLogPostTags(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT post_id, action, t.id, t.tag, n.id, n.nspace
		FROM log_post_tags pt
		JOIN log_post_tags_map ptm
		ON pt.id = ptm.ptid
		JOIN tags t
		ON ptm.tag_id = t.id
		JOIN namespaces n
		ON t.namespace_id = n.id
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			pid    int
			action lAction
			tag    = NewTag()
		)

		err = rows.Scan(
			&pid,
			&action,
			&tag.ID,
			&tag.Tag,
			&tag.Namespace.ID,
			&tag.Namespace.Namespace,
		)
		if err != nil {
			return err
		}

		ph := log.initPostHistory(pid)
		switch action {
		case aCreate:
			ph.Tags.Added = append(ph.Tags.Added, tag)
		case aDelete:
			ph.Tags.Removed = append(ph.Tags.Removed, tag)
		}
		ph.Tags.PostID = pid

		log.Posts[pid] = ph

	}

	return nil
}

type logPostTags struct {
	PostID  int
	Added   tagSet
	Removed tagSet
}

func (l logPostTags) log(logid int, tx *sql.Tx) error {
	createAction := func(action lAction, set tagSet) error {
		if len(set) <= 0 {
			return nil
		}

		var id int
		err := tx.QueryRow(`
			INSERT INTO log_post_tags (
				log_id,
				action,
				post_id
			)
			VALUES($1, $2, $3)
			RETURNING id
			`,
			logid,
			action,
			l.PostID,
		).Scan(&id)
		if err != nil {
			return err
		}

		stmt, err := tx.Prepare(`
			INSERT INTO log_post_tags_map (
				ptid,
				tag_id
			)
			VALUES($1, $2)
			`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, tag := range set {
			_, err = stmt.Exec(id, tag.ID)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if err := initPostLog(tx, logid, l.PostID); err != nil {
		return err
	}

	if err := createAction(aCreate, l.Added); err != nil {
		return err
	}
	return createAction(aDelete, l.Removed)
}

func getLogPostDescriptions(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT d.post_id, d.description, COALESCE((
			SELECT description
			FROM log_post_description
			WHERE log_id = (
				SELECT max(log_id)
				FROM log_post_description
				WHERE post_id = d.post_id
				AND log_id < d.log_id
			)
		), '') diff
		FROM log_post_description d
		WHERE d.log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var l logPostDescription
		err = rows.Scan(&l.PostID, &l.Description, &l.Old)
		if err != nil {
			return err
		}

		l.Diff = diffHtml(l.Old, l.Description)

		ph := log.initPostHistory(l.PostID)
		ph.Description = l
		log.Posts[l.PostID] = ph
	}

	return nil
}

type logPostDescription struct {
	PostID      int
	Description string
	Old         string
	Diff        string
}

func (l logPostDescription) log(logid int, tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO log_post_description (
			log_id,
			post_id,
			description
		)
		VALUES($1, $2, $3)
		`,
		logid,
		l.PostID,
		l.Description,
	)
	if err != nil {
		return err
	}

	return initPostLog(tx, logid, l.PostID)
}

func getLogPostMetaData(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT action, post_id, namespace, metadata
		FROM log_post_metadata
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var l logPostMetaData
		err = rows.Scan(&l.Action, &l.PostID, &l.Namespace, &l.MetaData)
		if err != nil {
			return err
		}

		ph := log.initPostHistory(l.PostID)
		ph.MetaData = append(ph.MetaData, l)
		log.Posts[l.PostID] = ph
	}

	return nil
}

type logPostMetaData struct {
	PostID    int
	Action    lAction
	Namespace string
	MetaData  string
}

func (l logPostMetaData) log(logid int, tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO log_post_metadata (
			log_id,
			action,
			post_id,
			namespace,
			metadata
		)
		VALUES($1,$2,$3,$4,$5)
		`,
		logid,
		l.Action,
		l.PostID,
		l.Namespace,
		l.MetaData,
	)
	if err != nil {
		return err
	}

	return initPostLog(tx, logid, l.PostID)
}

func getLogPostCreationDates(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT action, post_id, created
		FROM log_post_creation_dates
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var l logPostCreationDates
		err = rows.Scan(&l.Action, &l.postID, &l.Date)
		if err != nil {
			return err
		}

		ph := log.initPostHistory(l.postID)
		ph.CreationDates = append(ph.CreationDates, l)
		log.Posts[l.postID] = ph
	}

	return nil
}

type logPostCreationDates struct {
	postID int
	Action lAction
	Date   metaDate
}

func (l logPostCreationDates) log(logid int, tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO log_post_creation_dates (
			log_id,
			action,
			post_id,
			created
		)
		VALUES($1,$2,$3,$4)
		`,
		logid,
		l.Action,
		l.postID,
		l.Date.value(),
	)
	if err != nil {
		return err
	}

	return initPostLog(tx, logid, l.postID)
}
