package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"strconv"
	"sync"

	"github.com/Nr90/imgsim"
	"github.com/frustra/bbcode"

	shell "github.com/ipfs/go-ipfs-api"
	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/image"
	"github.com/kycklingar/PBooru/DataManager/sqlbinder"
	"github.com/kycklingar/PBooru/DataManager/timestamp"
	"github.com/kycklingar/PBooru/DataManager/user"
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
	p.Cid = cid
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
	Cid        string
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
	PFCid
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
	Cid  string
	Size int
}

func (p *Post) BindField(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case PFID:
		sel.Bind(&p.ID, "p.id", "")
	case PFCid:
		sel.Bind(&p.Cid, "p.multihash", "")
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
		sel.Rebind(&t.Cid)
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

	var (
		altIDs []int
		id     int
	)

	err := db.QueryRows(
		q,
		`SELECT id
		FROM posts
		WHERE alt_group = (
			SELECT alt_group
			FROM posts
			WHERE id = $1
		)
		ORDER BY id DESC`,
		p.ID,
	)(func(scan scanner) error {
		err := scan(&id)
		altIDs = append(altIDs, id)
		return err
	})
	if err != nil {
		return err
	}

	if len(altIDs) <= 1 {
		return nil
	}

	for _, id := range altIDs {
		var alt = NewPost()
		alt.ID = id
		if err = alt.QMul(
			q,
			PFCid,
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

	err := db.QueryRows(
		q,
		fmt.Sprintf(`
			SELECT %s
			FROM thumbnails
			WHERE post_id = $1
			`,
			selector.Select(),
		),
		p.ID,
	)(func(scan scanner) error {
		var thumb Thumb
		selector.ReBind(&thumb, fields...)
		err := scan(selector.Values()...)
		t = append(t, thumb)
		return err
	})
	if err != nil {
		return err
	}

	p.thumbnails = t

	return nil
}

func (p *Post) qMetaData(q querier, fields ...sqlbinder.Field) error {
	if !p.field(PFMetaData, fields...) {
		return nil
	}
	var metaMap = make(metaDataMap)

	err := db.QueryRows(
		q,
		`SELECT nspace, metadata
		FROM post_metadata
		JOIN namespaces
		ON namespaces.id = namespace_id
		WHERE post_id = $1`,
		p.ID,
	)(func(scan scanner) error {
		var m metadata
		err := scan(&m.namespace, &m.data)
		metaMap[string(m.namespace)] = append(metaMap[string(m.namespace)], m)
		return err
	})
	if err != nil {
		return err
	}

	err = db.QueryRows(
		q,
		`SELECT created
		FROM post_creation_dates
		WHERE post_id = $1`,
		p.ID,
	)(func(scan scanner) error {
		var t metaDate
		err := scan(&t)
		metaMap["date"] = append(metaMap["date"], t)
		return err
	})
	if err != nil {
		return err
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

func (p Post) ClosestThumbnail(size int) (ret string) {
	if len(p.thumbnails) <= 0 {
		return ""
	}
	var s int
	for _, k := range p.thumbnails {
		if k.Size > s {
			ret = k.Cid
			s = k.Size
		}
	}
	for _, k := range p.thumbnails {
		if k.Size < size {
			continue
		}
		if k.Size < s {
			s = k.Size
			ret = k.Cid
		}
	}
	return
}

func (p *Post) Vote(q querier, u user.User) error {
	if p.ID <= 0 {
		return errors.New("no post-id")
	}

	if u.ID <= 0 {
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

func (p *Post) SizePretty() string {
	const unit = 1024
	if p.Size < unit {
		return fmt.Sprintf("%d B", p.Size)
	}

	div, exp := int64(unit), 0
	for n := p.Size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %ciB", float64(p.Size)/float64(div), "kMGTPE"[exp])
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

func CreatePost(file io.ReadSeeker, user user.User, ud UploadData) (*Post, error) {
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
				thumb.Cid,
			); err != nil {
				return nil, err
			}

			err = store.Store(thumb.Cid, storeThumbnailDest(cid, thumb.Size))
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
func insertNewPost(file io.ReadSeeker, fsize int64, cid, mstr string, user user.User) (int, error) {
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

	tags, err := postTags(q, p.ID).unwrap()
	if err != nil {
		return err
	}

	for _, t := range tags {
		resetCacheTag(q, t.ID)
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

	tags, err := postTags(q, p.ID).unwrap()
	if err != nil {
		return err
	}

	for _, t := range tags {
		resetCacheTag(q, t.ID)
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

	err = p.QMul(tx, PFCid, PFThumbnails)
	if err != nil {
		return err
	}

	err = p.del(tx)
	if err != nil {
		return err
	}

	// Remove files from ipfs
	for _, thumb := range p.thumbnails {
		err = store.Remove(storeThumbnailDest(p.Cid, thumb.Size))
		if err != nil {
			return err
		}
	}

	err = store.Remove(storeFileDest(p.Cid))

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

	var phashes []phs

	err = db.QueryRows(
		q,
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
	)(func(scan scanner) error {
		var phn phs
		err = scan(&phn.postid, &phn.h1, &phn.h2, &phn.h3, &phn.h4)
		phashes = append(phashes, phn)
		return err
	})
	if err != nil {
		return nil, err
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

func (p *Post) Duplicates(q querier) (Dupe, error) {
	return getDupeFromPost(q, p)
}

func (p *Post) NewComment() PostComment {
	var pc PostComment
	pc.Post = p
	return pc
}

func (p *Post) Comments(q querier) ([]PostComment, error) {
	if p.ID <= 0 {
		return nil, nil
	}

	var pcs []PostComment

	return pcs, db.QueryRows(
		q,
		`SELECT id, user_id, text, timestamp
		FROM post_comments
		WHERE post_id = $1
		ORDER BY id DESC`,
		p.ID,
	)(func(scan scanner) error {
		var (
			text string
			pc   = p.NewComment()
			err  = scan(&pc.ID, &pc.User, &text, &pc.Time)
		)
		if err != nil {
			log.Println(err)
			return nil
		}

		cmp := bbcode.NewCompiler(true, true)
		cmp.SetTag("img", nil)
		pc.Text = cmp.Compile(text)

		pcs = append(pcs, pc)

		return nil
	})
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
