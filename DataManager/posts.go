package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Nr90/imgsim"
	"github.com/frustra/bbcode"

	shell "github.com/ipfs/go-ipfs-api"
	mm "github.com/kycklingar/MinMax"
	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/image"
	"github.com/kycklingar/PBooru/DataManager/sqlbinder"
	"github.com/kycklingar/PBooru/DataManager/timestamp"
	"github.com/kycklingar/sqhell/cond"
)

func NewPost() *Post {
	var p Post
	p.Mime = NewMime()
	//p.Deleted = -1
	p.Score = -1
	p.editCount = -1
	return &p
}

func GetPostFromCID(cid string) (*Post, error) {
	var p = NewPost()
	p.Hash = cid
	err := DB.QueryRow(`
		SELECT id FROM posts
		WHERE multihash = $1
		`,
		cid,
	).Scan(&p.ID)

	//if err != nil && err == sql.ErrNoRows {
	//	return p, nil
	//}

	return p, err
}

func GetPostFromHash(fnc, hash string) (*Post, error) {
	var p = NewPost()
	err := DB.QueryRow(fmt.Sprintf(`
			SELECT post_id
			FROM hashes
			WHERE %s = $1
			`,
		fnc,
	),
		hash,
	).Scan(&p.ID)

	//if err != nil && err == sql.ErrNoRows {
	//	return p, nil
	//}

	return p, err
}

func CachedPost(p *Post) *Post {
	if n := C.Cache.Get("PST", strconv.Itoa(p.ID)); n != nil {
		tp, ok := n.(*Post)
		if !ok {
			log.Println("cached variable not typeof *Post")
			return tp
		}
	} else {
		C.Cache.Set("PST", strconv.Itoa(p.ID), p)
	}

	return p
}

type metaDataMap map[string][]MetaData

func (m metaDataMap) merge(b metaDataMap) {
	for k, v := range b {
		m[k] = append(m[k], v...)
	}
}

type Post struct {
	ID         int
	Hash       string
	thumbnails []Thumb
	Mime       *Mime
	Removed    bool
	Deleted    bool
	Size       int64
	Dimension  Dimension
	Score      float64
	Timestamp  timestamp.Timestamp

	Checksums checksums

	AltGroup int
	Alts     []*Post
	MetaData metaDataMap

	Tombstone Tombstone

	Description string

	editCount int
}

const (
	sqlUpdatePostScores = `
		UPDATE posts
		SET score = (
			SELECT count(*) * 1000
			FROM post_score_mapping
			WHERE post_id = $1
		) + (
			SELECT COALESCE(SUM(views), 0)
			FROM post_views
			WHERE post_id = $1
		)
		WHERE id = $1
	`
)

const (
	PFID sqlbinder.Field = iota
	PFHash
	PFThumbnails
	PFMime
	PFRemoved
	PFDeleted
	PFSize
	PFDimension
	PFScore
	PFTimestamp
	PFChecksums
	PFAlts
	PFAltGroup
	PFDescription
	PFTombstone
	PFMetaData
)

type checksums struct {
	Sha256 string
	Md5    string
}

type Dimension struct {
	Width  int
	Height int
}

type Thumb struct {
	Hash string
	Size int
}

//type bindContext struct {
//	p     *Post
//	pinfo bool
//}
//
//func (b *bindContext) joinPostInfo() string {
//	if !b.pinfo {
//		b.pinfo = true
//		return "LEFT JOIN post_info ON p.id = post_info.post_id"
//	}
//
//	return ""
//}

func (p *Post) BindField(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case PFID:
		sel.Bind(&p.ID, "p.id", "")
	case PFHash:
		sel.Bind(&p.Hash, "p.multihash", "")
	case PFMime:
		sel.Bind(&p.Mime.Name, "m.mime", "LEFT JOIN mime_type m ON mime_id = m.id")
		sel.Bind(&p.Mime.Type, "m.type", "")
	case PFRemoved:
		sel.Bind(&p.Removed, "p.removed", "")
	case PFDeleted:
		sel.Bind(&p.Deleted, "p.deleted", "")
	case PFSize:
		sel.Bind(&p.Size, "p.file_size", "")
	case PFScore:
		sel.Bind(&p.Score, "p.score / 1000.0", "")
	case PFTimestamp:
		sel.Bind(&p.Timestamp, "p.timestamp", "")
	case PFAltGroup:
		sel.Bind(&p.AltGroup, "p.alt_group", "")
	case PFTombstone:
		sel.Bind(&p.Tombstone.Reason, "COALESCE(t.reason, '')", "LEFT JOIN tombstone t ON t.post_id = p.id")
		sel.Bind(&p.Tombstone.Removed, "COALESCE(t.removed, CURRENT_TIMESTAMP)", "")
	case PFDimension:
		sel.Bind(&p.Dimension.Width, "COALESCE(width, 0)", "LEFT JOIN post_info ON p.id = post_info.post_id")
		sel.Bind(&p.Dimension.Height, "COALESCE(height, 0)", "")
	case PFChecksums:
		sel.Bind(&p.Checksums.Sha256, "sha256", "LEFT JOIN hashes ON p.id = hashes.post_id")
		sel.Bind(&p.Checksums.Md5, "md5", "")
	case PFDescription:
		sel.Bind(&p.Description, "p.description", "")
	}
}

func (p *Post) QMul(q querier, fields ...sqlbinder.Field) error {
	selector := sqlbinder.BindFieldAddresses(p, fields...)

	query := fmt.Sprintf(`
		SELECT %s
		FROM posts p
		%s
		WHERE p.id = $1
		`,
		selector.Select(),
		selector.Joins(),
	)

	var c = make(chan error)

	go func() {
		c <- p.QThumbs(q, fields...)
	}()

	go func() {
		c <- p.QAlts(q, fields...)
	}()

	go func() {
		c <- p.qMetaData(q, fields...)
	}()

	go func() {
		c <- q.QueryRow(query, p.ID).Scan(selector.Values()...)
	}()

	var err error
	for i := 0; i < 4; i++ {
		er := <-c
		if er != nil {
			log.Println(er)
			err = er
		}
	}

	return err
}

type thumbnails []Thumb

func (t *Thumb) Rebind(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case PFThumbnails:
		sel.Rebind(&t.Hash)
		sel.Rebind(&t.Size)
	}
}

func (t thumbnails) BindField(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case PFThumbnails:
		sel.Bind(nil, "multihash", "")
		sel.Bind(nil, "dimension", "")
	}
}

func (p *Post) QAlts(q querier, fields ...sqlbinder.Field) error {
	if !p.field(PFAlts, fields...) {
		return nil
	}

	rows, err := q.Query(`
		SELECT id
		FROM posts
		WHERE alt_group = (
			SELECT alt_group
			FROM posts
			WHERE id = $1
		)
		ORDER BY id DESC
		`,
		p.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var altIDs []int

	var id int
	for rows.Next() {
		if err = rows.Scan(&id); err != nil {
			return err
		}

		altIDs = append(altIDs, id)
	}

	if len(altIDs) <= 1 {
		return nil
	}

	for _, id := range altIDs {
		var alt = NewPost()
		alt.ID = id
		if err = alt.QMul(
			q,
			PFHash,
			PFThumbnails,
		); err != nil {
			return err
		}

		p.Alts = append(p.Alts, alt)
	}

	return nil
}

func (p *Post) QThumbs(q querier, fields ...sqlbinder.Field) error {
	if !p.field(PFThumbnails, fields...) {
		return nil
	}

	var t thumbnails
	selector := sqlbinder.BindFieldAddresses(t, fields...)

	query := fmt.Sprintf(`
		SELECT %s
		FROM thumbnails
		WHERE post_id = $1
		`,
		selector.Select(),
	)

	rows, err := q.Query(query, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var thumb Thumb
		selector.ReBind(&thumb, fields...)
		err = rows.Scan(selector.Values()...)
		if err != nil {
			return err
		}

		t = append(t, thumb)
	}

	p.thumbnails = t

	return nil
}

func (p *Post) qMetaData(q querier, fields ...sqlbinder.Field) error {
	if !p.field(PFMetaData, fields...) {
		return nil
	}
	var metaMap = make(metaDataMap)

	err := func() error {
		rows, err := q.Query(`
		SELECT nspace, metadata
		FROM post_metadata
		JOIN namespaces
		ON namespaces.id = namespace_id
		WHERE post_id = $1
		`,
			p.ID,
		)
		if err != nil {
			log.Println(err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var m metadata
			err = rows.Scan(&m.namespace, &m.data)
			if err != nil {
				log.Println(err)
				return err
			}

			metaMap[string(m.namespace)] = append(metaMap[string(m.namespace)], m)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	rows, err := q.Query(`
		SELECT created
		FROM post_creation_dates
		WHERE post_id = $1
		`,
		p.ID,
	)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t metaDate
		if err = rows.Scan(&t); err != nil {
			log.Println(err)
			return err
		}

		metaMap["date"] = append(metaMap["date"], t)
	}

	p.MetaData = metaMap

	return nil
}

func (p *Post) field(field sqlbinder.Field, fields ...sqlbinder.Field) bool {
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}

func (p *Post) SetID(q querier, id int) error {
	return q.QueryRow("SELECT id FROM posts WHERE id=$1", id).Scan(&p.ID)
}

func (p Post) Thumbnails() []Thumb {
	var thumbs []Thumb
	for _, t := range p.thumbnails {
		if t.Size > 0 {
			thumbs = append(thumbs, t)
		}
	}
	return thumbs
}

func (p *Post) ClosestThumbnail(size int) (ret string) {
	if len(p.thumbnails) <= 0 {
		return ""
	}
	var s int
	for _, k := range p.thumbnails {
		if k.Size > s {
			ret = k.Hash
			s = k.Size
		}
	}
	for _, k := range p.thumbnails {
		if k.Size < size {
			continue
		}
		if k.Size < s {
			s = k.Size
			ret = k.Hash
		}
	}
	return
}

func (p *Post) Vote(q querier, u *User) error {
	if p.ID <= 0 {
		return errors.New("no post-id")
	}

	if u.QID(q) <= 0 {
		return errors.New("no user-id")
	}

	if _, err := q.Exec("SELECT FROM post_vote_update($1, $2)", p.ID, u.ID); err != nil {
		log.Println(err)
		return err
	}

	p.Score = -1

	return p.updateScore(q)
}

func (p *Post) updateScore(q querier) error {
	_, err := q.Exec(sqlUpdatePostScores, p.ID)
	return err
}

func (p *Post) QTagHistoryCount(q querier) (int, error) {
	if p.editCount >= 0 {
		return p.editCount, nil
	}

	if p.ID <= 0 {
		return 0, errors.New("no id specified")
	}

	err := q.QueryRow("SELECT count(*) FROM tag_history WHERE post_id = $1", p.ID).Scan(&p.editCount)

	return p.editCount, err
}

func (p *Post) TagHistory(q querier, limit, offset int) ([]*TagHistory, error) {
	if p.ID <= 0 {
		return nil, errors.New("no post id specified")
	}
	rows, err := q.Query("SELECT id, user_id, timestamp FROM tag_history WHERE post_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3", p.ID, limit, offset)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	var ths []*TagHistory

	for rows.Next() {
		var th = NewTagHistory()
		if err = rows.Scan(&th.ID, &th.User.ID, &th.Timestamp); err != nil {
			log.Println(err)
			return nil, err
		}

		th.Post = p

		ths = append(ths, th)
	}

	err = rows.Err()

	return ths, err
}

func (p *Post) SizePretty() string {
	const unit = 1000
	if p.Size < unit {
		return fmt.Sprintf("%dB", p.Size)
	}

	div, exp := int64(unit), 0
	for n := p.Size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f%cB", float64(p.Size)/float64(div), "KMGTPE"[exp])
}

func cidDir(cid string) string {
	c := len(cid) - 3
	return cid[c : c+2]
}

func storeFileDest(cid string) string {
	return path.Join("files", cidDir(cid), cid)
}

func storeThumbnailDest(cid string, size int) string {
	return path.Join("thumbnails", strconv.Itoa(size), cidDir(cid), cid)
}

var uploadQueue sync.Mutex

type UploadData struct {
	FileSize    int64
	TagStr      string
	Mime        string
	MetaData    string
	Description string
}

func CreatePost(file io.ReadSeeker, user *User, ud UploadData) (*Post, error) {
	uploadQueue.Lock()
	defer uploadQueue.Unlock()
	// Hash file
	// To prevent **DELETED** files from being added again

	// Undocumented behaviour in go-ipfs-api
	// shell.Add will close the file if it implements the Close interface
	cid, err := ipfs.Add(
		io.NopCloser(file),
		shell.CidVersion(1),
		shell.OnlyHash(true),
	)
	if err != nil {
		return nil, err
	}

	// Check if file already exists
	p, err := GetPostFromCID(cid)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// It's a new post
	if p.ID == 0 {
		var cid2 string

		if _, err = file.Seek(0, 0); err != nil {
			log.Println(err)
			return nil, err
		}

		cid2, err = ipfs.Add(
			io.NopCloser(file),
			shell.CidVersion(1),
			shell.Pin(false),
		)

		if cid2 != cid {
			return nil, fmt.Errorf("ipfs.add missmatch %s %s", cid, cid2)
		}

		// Store file
		err = store.Store(cid, storeFileDest(cid))
		if err != nil {
			return nil, err
		}

		// Create thumbnails
		file.Seek(0, 0)
		thumbs, err := makeThumbnails(file)
		if err != nil {
			log.Println(err)
		}

		if p.ID, err = insertNewPost(file, ud.FileSize, cid, ud.Mime, user); err != nil {
			return nil, err
		}

		for _, thumb := range thumbs {
			if _, err = DB.Exec(`
				INSERT INTO thumbnails(post_id, dimension, multihash)
				VALUES($1, $2, $3)
				`,
				p.ID,
				thumb.Size,
				thumb.Hash,
			); err != nil {
				return nil, err
			}

			err = store.Store(thumb.Hash, storeThumbnailDest(cid, thumb.Size))
			if err != nil {
				return nil, err
			}
		}

		totalPosts = 0
	}

	// Add tags
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer commitOrDie(tx, &err)

	var d Dupe
	d, err = getDupeFromPost(tx, p)
	if err != nil {
		return nil, err
	}

	ua := UserAction(user)
	ua.Add(AlterPostTags(d.Post.ID, ud.TagStr, ""))
	metalogs, err := PostAddMetaData(d.Post.ID, ud.MetaData)
	if err != nil {
		return nil, err
	}
	ua.Add(metalogs...)

	if ud.Description != "" {
		// Add description only if none exists already
		d.Post.QMul(tx, PFDescription)
		if d.Post.Description != "" {
			return p, errors.New("Failed to set description. An existing description is already set.")
		}
		ua.Add(PostChangeDescription(d.Post.ID, ud.Description))
	}

	err = ua.exec(tx)

	return p, err
}

// The following actions will take place
// Insert post
// Insert checksums
//
// Insert file dimensions if available
// Create a new mime if needed
// Generate appletrees
func insertNewPost(file io.ReadSeeker, fsize int64, cid, mstr string, user *User) (int, error) {
	var (
		postID        int
		mime          Mime
		err, dhErr    error
		u             imgsim.Hash
		width, height int
	)

	if err := mime.Parse(mstr); err != nil {
		return postID, err
	}

	// Do heavy tasks before starting a tx
	file.Seek(0, 0)
	sha, md := checksum(file)

	file.Seek(0, 0)
	width, height, err = image.GetDimensions(file)
	if err != nil {
		log.Println(err)
	}

	file.Seek(0, 0)
	u, dhErr = dHash(file)
	if dhErr != nil {
		log.Println(dhErr)
	}

	tx, err := DB.Begin()
	if err != nil {
		return postID, err
	}

	defer commitOrDie(tx, &err)

	mime.Save(tx)

	err = tx.QueryRow(`
		INSERT INTO posts(multihash, mime_id, uploader, file_size)
		VALUES($1, $2, $3, $4)
		RETURNING id
		`,
		cid,
		mime.ID,
		user.ID,
		fsize,
	).Scan(&postID)
	if err != nil {
		return postID, err
	}

	_, err = tx.Exec(`
		UPDATE posts
		SET alt_group = id
		WHERE id = $1
		`,
		postID,
	)
	if err != nil {
		return postID, err
	}

	_, err = tx.Exec(`
		INSERT INTO hashes(post_id, sha256, md5)
		VALUES($1, $2, $3)
		`,
		postID,
		sha,
		md,
	)
	if err != nil {
		return postID, err
	}

	if width > 0 && height > 0 {
		_, err = tx.Exec(`
			INSERT INTO post_info(post_id, width, height)
			VALUES($1, $2, $3)
			`,
			postID,
			width,
			height,
		)
		if err != nil {
			return postID, err
		}
	}

	if u > 0 {
		ph := phsFromHash(postID, u)

		err = ph.insert(tx)
		if err != nil {
			return postID, err
		}

		err = generateAppleTree(tx, ph)
	}

	return postID, err
}

func (p *Post) Remove(q querier) error {
	if p.ID == 0 {
		return errors.New("post:delete: invalid post")
	}
	_, err := q.Exec("UPDATE posts SET removed=true WHERE id=$1", p.ID)
	if err != nil {
		log.Print(err)
		return err

	}
	totalPosts = 0

	tc := TagCollector{}
	if err = tc.GetFromPost(q, p); err != nil {
		log.Print(err)
		return err
	}
	for _, t := range tc.Tags {
		resetCacheTag(q, t.QID(q))
	}
	C.Cache.Purge("PST", strconv.Itoa(p.ID))

	return clearEmptySearchCountCache(q)
}

func (p *Post) Reinstate(q querier) error {
	if p.ID == 0 {
		return errors.New("post:undelete: invalid post id")
	}
	_, err := q.Exec("UPDATE posts SET removed=false WHERE id=$1", p.ID)
	if err != nil {
		log.Print(err)
		return err
	}
	totalPosts = 0

	tc := TagCollector{}
	if err = tc.GetFromPost(q, p); err != nil {
		log.Print(err)
		return err
	}
	for _, t := range tc.Tags {
		resetCacheTag(q, t.QID(q))
	}
	C.Cache.Purge("PST", strconv.Itoa(p.ID))

	return clearEmptySearchCountCache(q)
}

// No going back
func (p *Post) Delete() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer commitOrDie(tx, &err)

	err = p.QMul(tx, PFHash, PFThumbnails)
	if err != nil {
		return err
	}

	err = p.del(tx)
	if err != nil {
		return err
	}

	// Remove files from ipfs
	for _, thumb := range p.thumbnails {
		err = store.Remove(storeThumbnailDest(p.Hash, thumb.Size))
		if err != nil {
			return err
		}
	}

	err = store.Remove(storeFileDest(p.Hash))

	return err
}

func (p *Post) del(q querier) error {

	// *Delete* from db
	// Want to keep a record of it
	// so it can't be readded later
	if _, err := q.Exec(`
		UPDATE posts
		SET deleted = true,
		removed = true
		WHERE id = $1
		`,
		p.ID,
	); err != nil {
		return err
	}

	// Delete thumbnails
	_, err := q.Exec(`
		DELETE FROM thumbnails
		WHERE post_id = $1
		`,
		p.ID,
	)

	return err
}

func (p *Post) addTags(tx querier, currentTags, tags []*Tag) ([]*Tag, error) {
	// Collect only new tags
	var newTags []*Tag
	for _, tag := range tags {
		if !isTagIn(tag, currentTags) {
			newTags = append(newTags, tag)
		}
	}

	for _, tag := range newTags {
		_, err := tx.Exec(`
			INSERT INTO post_tag_mappings(
				post_id, tag_id
			)
			VALUES ($1, $2)
			`,
			p.ID,
			tag.ID,
		)
		if err != nil {
			return nil, err
		}

		err = tag.updateCount(tx, 1)
		if err != nil {
			return nil, err
		}
	}

	return newTags, nil
}

func (p *Post) removeTags(tx querier, currentTags, tags []*Tag) ([]*Tag, error) {
	in := func(t *Tag, tags []*Tag) bool {
		for _, tag := range tags {
			if tag.ID == t.ID {
				return true
			}
		}

		return false
	}

	var removedTags []*Tag

	for _, tag := range currentTags {
		if in(tag, tags) {
			removedTags = append(removedTags, tag)
		}
	}

	for _, tag := range removedTags {
		_, err := tx.Exec(`
			DELETE FROM post_tag_mappings
			WHERE post_id = $1
			AND tag_id = $2
			`,
			p.ID,
			tag.ID,
		)
		if err != nil {
			return nil, err
		}

		err = tag.updateCount(tx, -1)
		if err != nil {
			return nil, err
		}
	}

	return removedTags, nil
}

func (p *Post) AddTags(user *User, tagStr string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	err = p.editTagsAdd(tx, user, tagStr)

	return err
}

func (p *Post) RemoveTags(user *User, tagStr string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	err = p.editTagsRemove(tx, user, tagStr)

	return err
}

func (p *Post) editTagsRemove(tx querier, user *User, tagStr string) error {
	var tags TagCollector
	err := tags.Parse(tagStr, "\n")
	if err != nil {
		return err
	}

	err = tags.upgrade(tx, false)
	if err != nil {
		return err
	}

	dupe, err := getDupeFromPost(tx, p)
	if err != nil {
		return err
	}

	var currentTags TagCollector
	if err = currentTags.GetFromPost(tx, dupe.Post); err != nil {
		return err
	}

	removedTags, err := dupe.Post.removeTags(tx, currentTags.Tags, tags.Tags)
	if err != nil {
		return err
	}

	if len(removedTags) <= 0 {
		return nil
	}

	if err = dupe.Post.logTagEdit(tx, user, nil, removedTags); err != nil {
		return err
	}

	for _, tag := range removedTags {
		resetCacheTag(tx, tag.ID)
	}

	C.Cache.Purge("TPC", strconv.Itoa(p.ID))

	return clearEmptySearchCountCache(tx)
}

func (p *Post) editTagsAdd(tx querier, user *User, tagStr string) error {

	var tags TagCollector
	err := tags.Parse(tagStr, "\n")
	if err != nil {
		return err
	}

	if err = tags.save(tx); err != nil {
		return err
	}

	// Get post dupe
	dupe, err := getDupeFromPost(tx, p)
	if err != nil {
		return err
	}

	// Upgrade tags to aliases and add parents
	err = tags.upgrade(tx, true)
	if err != nil {
		return err
	}

	// Get current tags on the post
	var currentTags TagCollector
	if err = currentTags.GetFromPost(tx, dupe.Post); err != nil {
		return err
	}

	newTags, err := dupe.Post.addTags(tx, currentTags.Tags, tags.Tags)
	if err != nil {
		return err
	}

	if len(newTags) <= 0 {
		return nil
	}

	if err = dupe.Post.logTagEdit(tx, user, newTags, nil); err != nil {
		return err
	}

	for _, tag := range newTags {
		resetCacheTag(tx, tag.ID)
	}

	C.Cache.Purge("TPC", strconv.Itoa(p.ID))

	return clearEmptySearchCountCache(tx)
}

func (p *Post) EditTagsQ(q querier, user *User, tagStr string) error {
	var tags TagCollector
	err := tags.Parse(tagStr, "\n")
	if err != nil {
		//log.Print(err)
	}

	if err = tags.save(q); err != nil {
		return err
	}

	dupe, err := getDupeFromPost(q, p)
	if err != nil {
		return err
	}

	// Upgrade to aliases

	err = tags.upgrade(q, true)
	if err != nil {
		return err
	}

	// Get current tags

	var currentTags TagCollector
	if err = currentTags.GetFromPost(q, dupe.Post); err != nil {
		return err
	}

	// Add tags to post

	newTags, err := dupe.Post.addTags(q, currentTags.Tags, tags.Tags)
	if err != nil {
		return err
	}

	// Remove tags not in tagStr
	var removedTags []*Tag

	for _, tag := range currentTags.Tags {
		if !isTagIn(tag, tags.Tags) {
			removedTags = append(removedTags, tag)
		}
	}

	removedTags, err = dupe.Post.removeTags(q, currentTags.Tags, removedTags)
	if err != nil {
		return err
	}

	if len(newTags) < 1 && len(removedTags) < 1 {
		return nil
		//return errors.New("no tags in edit")
	}

	if err = dupe.Post.logTagEdit(q, user, newTags, removedTags); err != nil {
		return err
	}

	for _, tag := range newTags {
		resetCacheTag(q, tag.ID)
	}

	for _, tag := range removedTags {
		resetCacheTag(q, tag.ID)
	}

	C.Cache.Purge("TPC", strconv.Itoa(p.ID))

	return clearEmptySearchCountCache(q)
}

func (p *Post) logTagEdit(tx querier, user *User, newTags, removedTags []*Tag) error {
	var historyID int

	err := tx.QueryRow(`
		INSERT INTO tag_history(
			user_id, post_id, timestamp
		)
		VALUES(
			$1, $2, CURRENT_TIMESTAMP
		)
		RETURNING id`,
		user.QID(tx),
		p.ID,
	).Scan(&historyID)
	if err != nil {
		return err
	}

	for _, tag := range newTags {
		_, err = tx.Exec(`
			INSERT INTO edited_tags(
				history_id, tag_id, direction
			)
			VALUES($1, $2, $3)
			`,
			historyID,
			tag.QID(tx),
			1,
		)
		if err != nil {
			return err
		}
	}

	for _, tag := range removedTags {
		_, err = tx.Exec(`
			INSERT INTO edited_tags(
				history_id, tag_id, direction
			)
			VALUES($1, $2, $3)
			`,
			historyID,
			tag.QID(tx),
			-1,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Post) EditTags(user *User, tagStr string) error {
	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}

	if err = p.EditTagsQ(tx, user, tagStr); err != nil {
		log.Println(err)
		return txError(tx, err)
	}

	return tx.Commit()
}

func (p *Post) FindSimilar(q querier, dist int, removed bool) ([]*Post, error) {
	if p.ID == 0 {
		return nil, errors.New("id = 0")
	}

	var rem string
	if !removed {
		rem = "AND removed = FALSE"
	}

	var ph phs

	err := q.QueryRow("SELECT * FROM phash WHERE post_id = $1", p.ID).Scan(&ph.postid, &ph.h1, &ph.h2, &ph.h3, &ph.h4)
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(
		fmt.Sprintf(
			`
			SELECT post_id, h1, h2, h3, h4
			FROM phash
			JOIN posts
			ON post_id = id
			WHERE (
				h1=$1
				OR h2=$2
				OR h3=$3
				OR h4=$4
			)
			%s
			ORDER BY post_id DESC
			`,
			rem,
		),
		ph.h1,
		ph.h2,
		ph.h3,
		ph.h4,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phashes []phs

	for rows.Next() {
		var phn phs
		if err = rows.Scan(&phn.postid, &phn.h1, &phn.h2, &phn.h3, &phn.h4); err != nil {
			return nil, err
		}
		phashes = append(phashes, phn)
	}

	var posts []*Post

	for _, h := range phashes {
		if ph.distance(h) < dist {
			pst := NewPost()
			pst.ID = h.postid
			posts = append(posts, pst)
		}
	}

	return posts, nil
}

func (p *Post) Chapters(q querier) []*Chapter {
	if p.ID == 0 {
		return nil
	}

	rows, err := q.Query("SELECT chapter_id FROM comic_mappings WHERE post_id = $1", p.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println(err)
		}
		return nil
	}

	defer rows.Close()
	var chapters []*Chapter

	in := func(id int) bool {
		for _, chapter := range chapters {
			if chapter.ID == id {
				return true
			}
		}

		return false
	}

	for rows.Next() {
		var c = new(Chapter)
		if err := rows.Scan(&c.ID); err != nil {
			log.Println(err)
			return nil
		}

		if !in(c.ID) {
			chapters = append(chapters, c)
		}
	}

	if err = rows.Err(); err != nil {
		log.Println(err)
		return nil
	}

	return chapters
}

func (p *Post) Comics(q querier) []*Comic {
	if p.ID == 0 {
		return nil
	}
	rows, err := q.Query("SELECT comic_id FROM comic_mappings WHERE post_id=$1", p.ID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var comics []*Comic
	for rows.Next() {
		var c Comic
		rows.Scan(&c.ID)
		if rows.Err() != nil {
			log.Print(err)
			return nil
		}
		comics = append(comics, &c)
	}
	return comics
}

func (p *Post) Duplicates(q querier) (Dupe, error) {
	return getDupeFromPost(q, p)
}

func (p *Post) NewComment() *PostComment {
	pc := newPostComment()
	pc.Post = p
	return pc
}

func (p *Post) Comments(q querier) []*PostComment {
	if p.ID <= 0 {
		return nil
	}

	rows, err := q.Query(`
		SELECT id, user_id, text, timestamp
		FROM post_comments
		WHERE post_id = $1
		ORDER BY id DESC`,
		p.ID,
	)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()

	var pcs []*PostComment

	for rows.Next() {
		pc := p.NewComment()
		var text string
		err = rows.Scan(&pc.ID, &pc.User.ID, &text, &pc.Time)
		if err != nil {
			log.Println(err)
			return nil
		}

		cmp := bbcode.NewCompiler(true, true)
		cmp.SetTag("img", nil)
		pc.Text = cmp.Compile(text)

		pcs = append(pcs, pc)
	}

	return pcs
}

func NewPostCollector() *PostCollector {
	return &PostCollector{TotalPosts: -1}
}

type PostCollector struct {
	emptySet bool

	posts     map[int][]*Post
	id        []int
	or        []int
	filter    []int
	unless    []int
	tombstone bool

	tags       []*Tag //Sidebar
	TotalPosts int

	mimeIDs []int
	order   string

	altGroup    int
	collectAlts bool

	tagLock sync.Mutex
	pl      sync.RWMutex
}

var perSlice = 500

func CachedPostCollector(pc *PostCollector) *PostCollector {
	c := C.Cache.Get("PC", pc.idStr())
	if c != nil {
		return c.(*PostCollector)
	}

	C.Cache.Set("PC", pc.idStr(), pc)
	return pc
}

type SearchOptions struct {
	And    string
	Or     string
	Filter string
	Unless string

	MimeIDs    []int
	Altgroup   int
	AltCollect bool
	Tombstone  bool
	Order      string
}

type ErrorTag string

func (e ErrorTag) Error() string {
	return "tag does not exist: " + string(e)
}

func (pc *PostCollector) Get(opts SearchOptions) error {
	in := func(i int, arr []int) bool {
		for _, j := range arr {
			if i == j {
				return true
			}
		}
		return false
	}

	type strat int
	const (
		stratContinue strat = iota
		stratError
	)

	addto := func(arr *[]int, tstr string, tagNoExistStrat strat) error {
		var isin = make(map[int]struct{})
		if len(tstr) >= 1 {
			var tc TagCollector
			err := tc.ParseEscape(tstr, ',')
			if err != nil {
				return err
			}

			for _, tag := range tc.Tags {
				if tag.QID(DB) == 0 {
					switch tagNoExistStrat {
					case stratError:
						return ErrorTag(tag.String())
					default:
						continue
					}

				}

				alias := NewAlias()
				alias.Tag = tag
				to, err := alias.QTo(DB)
				if err != nil {
					return err
				}

				if to.QID(DB) != 0 {
					tag = to
				}

				if _, in := isin[tag.QID(DB)]; !in {
					isin[tag.ID] = struct{}{}
					*arr = append(*arr, tag.ID)
				}
			}
		}

		sort.Ints(*arr)

		return nil
	}

	if err := addto(&pc.id, opts.And, stratError); err != nil {
		pc.emptySet = true
		return err
	}

	if err := addto(&pc.or, opts.Or, stratError); err != nil {
		pc.emptySet = true
		return err
	}

	if err := addto(&pc.filter, opts.Filter, stratError); err != nil {
		pc.emptySet = true
		return err
	}

	if err := addto(&pc.unless, opts.Unless, stratError); err != nil {
		pc.emptySet = true
		return err
	}

	switch strings.ToUpper(opts.Order) {
	case "ASC", "DESC", "SCORE":
		pc.order = strings.ToUpper(opts.Order)
	case "RANDOM":
		pc.order = strings.ToUpper(opts.Order) + "()"
	default:
		pc.order = "DESC"
	}

	pc.collectAlts = opts.AltCollect
	pc.altGroup = opts.Altgroup
	pc.tombstone = opts.Tombstone

	// Check if the mime id exist in the db
	for _, mime := range Mimes {
		if in(mime.ID, opts.MimeIDs) {
			pc.mimeIDs = append(pc.mimeIDs, mime.ID)
		}
	}
	sort.Ints(pc.mimeIDs)

	return nil
}

func strSep(values []int, sep string) string {
	var ret string
	for i, v := range values {
		ret += fmt.Sprint(v)
		if i < len(values)-1 {
			ret += sep
		}
	}

	return ret
}

func sepStr(sep string, values ...string) string {
	var ret string
	for i, v := range values {
		ret += fmt.Sprint(v)
		if i < len(values)-1 {
			ret += sep
		}
	}

	return ret
}

func (pc *PostCollector) countIDStr() string {
	if pc.emptySet {
		return "NIL"
	}

	if !pc.tombstone && len(pc.id) <= 0 && len(pc.or) <= 0 && len(pc.filter) <= 0 && len(pc.mimeIDs) <= 0 && !pc.collectAlts && pc.altGroup <= 0 {
		return "0"
	}

	return fmt.Sprint(
		strSep(pc.id, " "),
		" - ",
		strSep(pc.or, " "),
		" - ",
		strSep(pc.filter, " "),
		" - ",
		strSep(pc.unless, " "),
		" - ",
		strSep(pc.mimeIDs, " "),
		" - ",
		pc.altGroup,
		" - ",
		pc.collectAlts,
		" - ",
		pc.tombstone,
	)
}

func (pc *PostCollector) idStr() string {
	if pc.emptySet {
		return "NIL"
	}

	return fmt.Sprint(
		pc.countIDStr(),
		" - ",
		pc.order,
	)
}

//type SearchResult struct {
//	Posts []*Post
//	Tags map[int][]*Tag
//}

type SearchResult []resultSet

type resultSet struct {
	Post *Post
	Tags []*Tag
}

// Where key is post id and val is tags
//type sRes map[int][]int

func (pc *PostCollector) Search2(limit, offset int) (SearchResult, error) {
	//var result = SearchResult{Tags:make(map[int][]*Tag)}
	var result SearchResult

	if pc.emptySet {
		pc.TotalPosts = 0
		return result, nil
	}

	sg := searchGroup(
		pc.id,
		pc.or,
		pc.filter,
		pc.unless,
		pc.tombstone,
	)

	var (
		order    string
		altGroup string
		mimes    string

		//wgr []string = []string{"p.removed = false"}
		wgr = cond.NewGroup().Add("", cond.N("p.removed = false"))

		sel string
	)

	if len(pc.mimeIDs) > 0 {
		for i, mi := range pc.mimeIDs {
			mimes += fmt.Sprint(mi)
			if i < len(pc.mimeIDs)-1 {
				mimes += ","
			}
		}
		mimes = fmt.Sprintf("p.mime_id IN(%s) ", mimes)
		//wgr = append(wgr, mimes)
		wgr.Add("\nAND", cond.N(mimes))
	}

	switch pc.order {
	case "RANDOM()":
		order = "RANDOM()"
		offset = 0
	case "SCORE":
		order = fmt.Sprint("p.score DESC, p.id DESC")
	default:
		order = "p.id " + pc.order
	}

	if pc.altGroup > 0 {
		altGroup = fmt.Sprintf(`
			p.alt_group = (
				SELECT alt_group
				FROM posts
				WHERE id = %d
			)
			`, pc.altGroup)
		//wgr = append(wgr, altGroup)
		wgr.Add("\nAND", cond.N(altGroup))
	}

	sel = sg.sel(wgr)

	// TODO: refactor
	if pc.TotalPosts < 0 {
		if pc.countIDStr() != "0" {
			var query string
			if pc.collectAlts {
				query = `
					SELECT COUNT(DISTINCT p.alt_group)
					FROM %s
					`
			} else {
				query = `
					SELECT count(p.id)
					FROM %s
					`
			}
			c := pc.ccGet()
			if c < 0 {
				query := fmt.Sprintf(
					query,
					sel,
				)

				//fmt.Println(query)

				err := DB.QueryRow(query).Scan(&pc.TotalPosts)
				if err != nil {
					log.Println(err)
					return result, err
				}
				pc.ccSet(pc.TotalPosts)
			} else {
				pc.TotalPosts = c
			}
		} else {
			pc.TotalPosts = GetTotalPosts()
		}
	}

	// No more posts beyond this point
	if pc.TotalPosts <= 0 || offset > pc.TotalPosts {
		return result, nil
	}

	var query string
	if pc.collectAlts {
		query = `
			SELECT id FROM (
				SELECT DISTINCT ON (p.alt_group) p.*
				FROM %s
				ORDER BY p.alt_group, p.score DESC, p.id DESC
			) AS p
			ORDER BY %s
			LIMIT $1
			OFFSET $2
			`
	} else {
		query = `
			SELECT p.id
			FROM %s
			ORDER BY %s
			LIMIT $1
			OFFSET $2
			`
	}

	query = fmt.Sprintf(
		fmt.Sprintf(`
				WITH res AS (
					%s
				)

				SELECT p.id, ptm.tag_id, t.tag, t.count, n.nspace
				FROM posts p
				LEFT JOIN post_tag_mappings ptm
				JOIN tags t
				JOIN namespaces n
				ON n.id = t.namespace_id
				ON t.id = ptm.tag_id
				ON p.id = ptm.post_id
				JOIN res
				ON res.id = p.id
				ORDER BY %s
			`,
			query,
			order,
		),
		sel,
		order,
	)

	//fmt.Println(query)

	rows, err := DB.Query(query, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	//var tmpPosts []*Post

	//var result make(sRes)

	var collectorFunc func(rows *sql.Rows) (SearchResult, error)

	if order == "RANDOM()" {
		collectorFunc = func(rows *sql.Rows) (SearchResult, error) {
			var pmap = make(map[int]resultSet)

			for rows.Next() {
				var (
					tagID     sql.NullInt64
					tagCount  sql.NullInt64
					tagName   sql.NullString
					namespace sql.NullString
					post      = NewPost()
				)

				rows.Scan(&post.ID, &tagID, &tagName, &tagCount, &namespace)
				if err != nil {
					return result, err
				}

				var (
					set resultSet
					ok  bool
				)

				if set, ok = pmap[post.ID]; !ok {
					set = resultSet{Post: post}
				}

				if tagID.Valid {
					var t = NewTag()
					t.ID = int(tagID.Int64)
					t.Count = int(tagCount.Int64)
					t.Tag = tagName.String
					t.Namespace.Namespace = namespace.String
					set.Tags = append(set.Tags, t)
				}

				pmap[post.ID] = set
			}

			var result = make(SearchResult, len(pmap))

			var i int
			for _, set := range pmap {
				result[i] = set
				i++
			}

			return result, rows.Err()
		}
	} else {
		collectorFunc = func(rows *sql.Rows) (SearchResult, error) {
			var (
				res      SearchResult
				prev     int
				resCount int
			)

			for rows.Next() {
				var (
					tagID     sql.NullInt64
					tagCount  sql.NullInt64
					tagName   sql.NullString
					namespace sql.NullString

					post = NewPost()
				)

				err := rows.Scan(&post.ID, &tagID, &tagName, &tagCount, &namespace)
				if err != nil {
					return result, err
				}

				if prev != post.ID {
					res = append(res, resultSet{Post: post})
					prev = post.ID
					resCount++
				}

				if tagID.Valid {
					var t = NewTag()
					t.ID = int(tagID.Int64)
					t.Count = int(tagCount.Int64)
					t.Tag = tagName.String
					t.Namespace.Namespace = namespace.String
					res[resCount-1].Tags = append(res[resCount-1].Tags, t)

				}
			}

			return res, rows.Err()
		}
	}

	result, err = collectorFunc(rows)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (pc *PostCollector) Tags(maxTags int) []*Tag {
	pc.tagLock.Lock()
	defer pc.tagLock.Unlock()
	if len(pc.tags) > 0 {
		return pc.tags
	}

	var allTags []*Tag

	if pc.countIDStr() == "-1" {
		return nil
	}

	// Get the first batch
	result, err := pc.Search2(500, 0)
	if err != nil {
		log.Println(err)
		return nil
	}

	// Get tags from all posts
	pc.pl.RLock()
	for _, set := range result {
		//var ptc TagCollector
		//err := ptc.GetFromPost(DB, set.Post)
		//if err != nil {
		//	continue
		//}
		//allTags = append(allTags, ptc.Tags...)
		allTags = append(allTags, set.Tags...)
	}
	pc.pl.RUnlock()

	type tagMap struct {
		tag   *Tag
		count int
	}

	tm := make(map[int]*tagMap)

	for _, tag := range allTags {
		if t, ok := tm[tag.ID]; ok {
			t.count++
		} else {
			tm[tag.ID] = &tagMap{tag, 1}
		}
	}

	var countMap = make([]tagMap, len(tm))
	var i int
	for _, v := range tm {
		countMap[i] = *v
		i++
	}

	sort.Slice(countMap, func(i, j int) bool {
		return countMap[i].count > countMap[j].count
	})

	// Hotfix for when cache is gc'd and there are multiple calls for this search
	pc.tags = nil
	// Retrive and append the tags
	//arrLimit := maxTags
	//if len(countMap) < arrLimit {
	//	arrLimit = len(countMap)
	//}
	arrLimit := mm.Min(maxTags, len(countMap))
	for i := 0; i < arrLimit; i++ {
		//tag := CachedTag(countMap[i].tag)
		pc.tags = append(pc.tags, countMap[i].tag)
	}

	return pc.tags
}

var totalPosts int

func GetTotalPosts() int {
	if totalPosts != 0 {
		return totalPosts
	}
	err := DB.QueryRow("SELECT count(*) FROM posts WHERE removed=false").Scan(&totalPosts)
	if err != nil {
		log.Println(err)
		return totalPosts
	}
	return totalPosts
}

func clearEmptySearchCountCache(q querier) error {
	_, err := q.Exec(`
		DELETE FROM search_count_cache
		WHERE id IN(
			SELECT id
			FROM search_count_cache
			LEFT JOIN search_count_cache_tag_mapping
			ON id = cache_id
			WHERE cache_id IS NULL
		)
		`,
	)

	return err
}

func resetCacheTag(q querier, tagID int) {
	C.Cache.Purge("PC", strconv.Itoa(tagID))
	C.Cache.Purge("TAG", strconv.Itoa(tagID))
	C.Cache.Purge("PC", "0")
	ccPurge(q, tagID)
}

func ccPurge(q querier, tagID int) {
	_, err := q.Exec("DELETE FROM search_count_cache WHERE id IN(SELECT cache_id FROM search_count_cache_tag_mapping WHERE tag_id = $1)", tagID)
	if err != nil {
		log.Println(err)
	}
}

func (pc *PostCollector) ccGet() (c int) {
	if err := DB.QueryRow("SELECT count FROM search_count_cache WHERE str = $1", pc.countIDStr()).Scan(&c); err != nil {
		return -1
	}
	return
}

func (pc *PostCollector) ccSet(c int) {
	if c < 0 {
		return
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return
	}

	var cid int
	err = tx.QueryRow(`
		INSERT INTO search_count_cache (str, count)
		VALUES($1, $2)
		RETURNING id
		`,
		pc.countIDStr(),
		c,
	).Scan(&cid)
	if err != nil {
		log.Println(err)
		txError(tx, err)
		return
	}

	in := func(id int, sl []int) bool {
		for _, i := range sl {
			if id == i {
				return true
			}
		}
		return false
	}

	var tagids []int
	for _, id := range pc.id {
		if !in(id, tagids) {
			tagids = append(tagids, id)
		}
	}
	for _, id := range pc.or {
		if !in(id, tagids) {
			tagids = append(tagids, id)
		}
	}
	for _, id := range pc.filter {
		if !in(id, tagids) {
			tagids = append(tagids, id)
		}
	}
	for _, id := range pc.unless {
		if !in(id, tagids) {
			tagids = append(tagids, id)
		}
	}

	for _, id := range tagids {
		tx.Exec("INSERT INTO search_count_cache_tag_mapping (cache_id, tag_id) VALUES($1, $2)", cid, id)
		if err != nil {
			log.Println(err)
			txError(tx, err)
			return
		}
	}
	tx.Commit()

	return
}

func (p *PostCollector) set(offset int, posts []*Post) {
	p.pl.Lock()
	defer p.pl.Unlock()
	if p.posts == nil {
		p.posts = make(map[int][]*Post)
	}
	//fmt.Println("Setting Offset = ", offset)
	p.posts[offset] = posts
}
