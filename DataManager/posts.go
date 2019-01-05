package DataManager

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Nr90/imgsim"
	"github.com/frustra/bbcode"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

func NewPost() *Post {
	var p Post
	p.Mime = NewMime()
	p.Deleted = -1
	return &p
}

type Thumb struct{
	Hash string
	Size int
}


type Post struct {
	ID         int
	Hash       string
	Thumbnails []Thumb
	Mime       *Mime
	Deleted    int
}

func (p *Post) QID(q querier) int {
	if p.ID != 0 {
		return p.ID
	}

	if p.Hash != "" {
		err := q.QueryRow("SELECT id FROM posts WHERE multihash=$1", p.Hash).Scan(&p.ID)
		if err != nil && err != sql.ErrNoRows {
			log.Print(err)
			return 0
		}
	}

	//C.Cache.Set("PST", strconv.Itoa(p.QID()), p)

	return p.ID
}

func (p *Post) SetID(q querier, id int) error {
	return q.QueryRow("SELECT id FROM posts WHERE id=$1", id).Scan(&p.ID)
}

func (p *Post) QHash(q querier) string {
	if p.Hash != "" {
		return p.Hash
	}

	if p.ID != 0 {
		// if t := C.Cache.Get(p.ID); t != nil {
		// 	p = t.(*Post)
		// 	return p.Hash
		// }
		if c := C.Cache.Get("PST", strconv.Itoa(p.ID)); c != nil {
			switch cp := c.(type) {
			case *Post:
				*p = *cp
				if p.Hash != "" {
					return p.Hash
				}
			}
		}

		err := q.QueryRow("SELECT multihash FROM posts WHERE id=$1", p.ID).Scan(&p.Hash)
		if err != nil {
			log.Print(err)
			return ""
		}
		C.Cache.Set("PST", strconv.Itoa(p.ID), p)
	}

	return p.Hash
}

func (p *Post) QThumbnails(q querier) error {
	if len(p.Thumbnails) > 0{
		return nil
	}
	if p.QID(q) == 0 {
		return errors.New("nil id")
	}

	rows, err := q.Query("SELECT multihash, dimension FROM thumbnails WHERE post_id=$1", p.ID)
	if err != nil {
		log.Print(err)
	}
	defer rows.Close()

	for rows.Next() {
		var t Thumb
		if err = rows.Scan(&t.Hash, &t.Size); err != nil {
			log.Println(err)
			return err
		}
		p.Thumbnails = append(p.Thumbnails, t)
	}

	p.Thumbnails = append(p.Thumbnails, Thumb{Hash:"", Size:0})
	return rows.Err()
}

func (p *Post) ClosestThumbnail(size int) (ret string) {
	if len(p.Thumbnails) <= 0 {
		return ""
	}
	var s int
	for _, k := range p.Thumbnails {
		if k.Size > s {
			s = k.Size
		}
	}
	for _, k := range p.Thumbnails {
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

func (p *Post) QMime(q querier) *Mime {
	if p.Mime.QID(q) != 0 {
		return p.Mime
	}
	err := q.QueryRow("SELECT mime_id FROM posts WHERE id=$1", p.QID(q)).Scan(&p.Mime.ID)
	if err != nil {
		log.Print(err)
	}

	return p.Mime
}

func (p *Post) QDeleted(q querier) int {
	if p.Deleted != -1 {
		return p.Deleted
	}
	if p.QID(q) == 0 {
		return -1
	}
	var deleted bool
	err := q.QueryRow("SELECT deleted FROM posts WHERE id=$1", p.ID).Scan(&deleted)
	if err != nil {
		log.Print(err)
		return -1
	}

	if deleted {
		p.Deleted = 1
	} else {
		p.Deleted = 0
	}

	//C.Cache.Set("PST", strconv.Itoa(p.QID()), p)

	return p.Deleted
}

func (p *Post) New(file io.ReadSeeker, tagString, mime string, user *User) error {
	var err error
	p.Hash, err = ipfsAdd(file)
	if err != nil {
		log.Println("Error pinning file to ipfs: ", err)
		return err
	}
	file.Seek(0, 0)

	if err = mfsCP(CFG.MFSRootDir+"files/", p.Hash, true); err != nil {
		log.Println("Error copying file to mfs: ", err)
		return err
	}

	var tx *sql.Tx

	if p.QID(DB) == 0 {

		for _, dim := range CFG.ThumbnailSizes{
			thash, err := makeThumbnail(file, mime, dim)
			if err != nil {
				return err
			}
			p.Thumbnails = append(p.Thumbnails, Thumb{Hash:thash, Size:dim})
		}

		tx, err = DB.Begin()
		if err != nil {
			log.Println("Error creating transaction: ", err)
			return err
		}
		//p.setQ(tx)

		err = p.Mime.Parse(mime)
		if err != nil {
			log.Println(err)
			return txError(tx, err)
		}

		if p.Mime.QID(tx) == 0 {
			err := p.Mime.Save(tx)
			if err != nil {
				log.Println(err)
				return txError(tx, err)
			}
		}

		err = p.Save(tx, user)
		if err != nil {
			log.Println(err)
			return txError(tx, err)
		}

		if p.Mime.Type == "image" {
			f := ipfsCat(p.Hash)
			u := dHash(f)
			f.Close()

			type PHS struct {
				id int
				h1 uint16
				h2 uint16
				h3 uint16
				h4 uint16
			}
			var ph PHS

			ph.id = p.QID(tx)

			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, u)
			ph.h1 = uint16(b[1]) | uint16(b[0])<<8
			ph.h2 = uint16(b[3]) | uint16(b[2])<<8
			ph.h3 = uint16(b[5]) | uint16(b[4])<<8
			ph.h4 = uint16(b[7]) | uint16(b[6])<<8

			_, err = tx.Exec("INSERT INTO phash(post_id, h1, h2, h3, h4) VALUES($1, $2, $3, $4, $5)", ph.id, ph.h1, ph.h2, ph.h3, ph.h4)
			if err != nil {
				return txError(tx, err)
			}
		}
	} else {
		tx, err = DB.Begin()
		if err != nil {
			log.Println("Error creating transaction: ", err)
			return err
		}

		err = p.EditTagsQ(tx, user, tagString, "")
		if err != nil && err.Error() != "no tags in edit" {
			//log.Println(err)
			return txError(tx, err)
		}

		err = tx.Commit()
		return err

		//p.setQ(tx)

		// Get the best post and add the tags to it
		// d := NewDuplicate()
		// d.setQ(tx)
		// d.Post = p
		// post = d.BestPost()
	}

	var tc TagCollector

	err = tc.Parse(tagString)
	if err != nil && err.Error() != "error decoding any tags" {
		return txError(tx, err)
	}

	err = tc.AddToPost(tx, p)
	if err != nil {
		return txError(tx, err)
	}

	err = tx.Commit()

	//p.setQ(nil)

	return err
}

func (p *Post) Save(q querier, user *User) error {
	if p.QID(q) != 0 {
		return errors.New("post already exist")
	}

	if p.Mime.QID(q) == 0 {
		err := p.Mime.Save(q)
		if err != nil {
			return err
		}
	}

	if p.Hash == "" || p.Mime.QID(q) == 0 || user.QID(q) == 0 {
		return fmt.Errorf("post missing argument. Want Hash and Mime.ID, Have: %s, %d, %d", p.Hash, p.Mime.ID, user.ID)
	}

	_, err := q.Exec("INSERT INTO posts(multihash, mime_id, uploader) VALUES($1, $2, $3, $4)", p.Hash, p.Mime.QID(q), user.QID(q))
	if err != nil {
		log.Print(err)
		return err
	}
	for _, t := range p.Thumbnails{
		_, err = q.Exec("INSERT INTO thumbnails (post_id, dimension, multihash) VALUES($1, $2, $3)", p.ID, t.Size, t.Hash)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	totalPosts = 0

	return nil
}

func (p *Post) Delete(q querier) error {
	if p.QID(q) == 0 {
		return errors.New("post:delete: invalid post")
	}
	_, err := q.Exec("UPDATE posts SET deleted=true WHERE id=$1", p.QID(q))
	if err != nil {
		log.Print(err)
		return err

	}
	totalPosts = 0

	tc := TagCollector{}
	if err = tc.GetFromPost(q, *p); err != nil {
		log.Print(err)
		return err
	}
	for _, t := range tc.Tags {
		resetCacheTag(t.QID(q))
	}
	C.Cache.Purge("PST", strconv.Itoa(p.QID(q)))

	return nil
}

func (p *Post) UnDelete(q querier) error {
	if p.QID(q) == 0 {
		return errors.New("post:undelete: invalid post id")
	}
	_, err := q.Exec("UPDATE posts SET deleted=false WHERE id=$1", p.QID(q))
	if err != nil {
		log.Print(err)
		return err
	}
	totalPosts = 0

	tc := TagCollector{}
	if err = tc.GetFromPost(q, *p); err != nil {
		log.Print(err)
		return err
	}
	for _, t := range tc.Tags {
		resetCacheTag(t.QID(q))
	}
	C.Cache.Purge("PST", strconv.Itoa(p.QID(q)))

	return nil
}

func (p *Post) EditTagsQ(q querier, user *User, tagStrAdd, tagStrRem string) error {
	var addTags TagCollector
	err := addTags.Parse(tagStrAdd)
	if err != nil {
		//log.Print(err)
	}
	var remTags TagCollector
	err = remTags.Parse(tagStrRem)
	if err != nil {
		//log.Print(err)
	}

	if len(addTags.Tags) < 1 && len(remTags.Tags) < 1 {
		return errors.New("no tags in edit")
	}

	var at []*Tag
	for _, t := range addTags.Tags {
		a := NewAlias()
		a.Tag = t
		//a.setQ(tx)
		b := a.QTo(q)
		if b.QID(q) == 0 {
			b = t
		}

		if b.QID(q) == 0 {
			err = b.Save(q)
			if err != nil {
				log.Print(err)
				return err
			}
		}

		var tmp int
		err = q.QueryRow("SELECT count(1) FROM post_tag_mappings WHERE post_id=$1 AND tag_id=$2", p.QID(q), b.QID(q)).Scan(&tmp)
		if err == nil && tmp > 0 {
			continue // Tag is already on this post
		}

		at = append(at, b)

	}

	var rt []*Tag
	for _, t := range remTags.Tags {
		a := NewAlias()
		a.Tag = t
		b := a.QTo(q)
		if b.QID(q) == 0 {
			b = t
		}

		if b.QID(q) == 0 {
			continue
		}

		var exist int
		err := q.QueryRow("SELECT count(1) FROM post_tag_mappings WHERE post_id=$1 AND tag_id=$2", p.QID(q), b.QID(q)).Scan(&exist)
		if err != nil || exist == 0 {
			// Tag does not exist on this post
			continue
		}

		rt = append(rt, b)
	}

	if len(at) < 1 && len(rt) < 1 {
		return errors.New("no tags in edit")
	}

	addTags.Tags = at
	remTags.Tags = rt

	res, err := q.Exec("INSERT INTO tag_history(user_id, post_id, timestamp) VALUES($1, $2, CURRENT_TIMESTAMP)", user.QID(q), p.QID(q))
	if err != nil {
		return err
	}
	id64, err := res.LastInsertId()

	historyID := int(id64)

	for _, tag := range at {
		_, err = q.Exec("INSERT INTO edited_tags(history_id, tag_id, direction) VALUES($1, $2, $3)", historyID, tag.QID(q), 1)
		if err != nil {
			return err
		}
	}
	addTags.AddToPost(q, p)

	for _, tag := range rt {
		_, err = q.Exec("INSERT INTO edited_tags(history_id, tag_id, direction) VALUES($1, $1, $3)", historyID, tag.QID(q), -1)
		if err != nil {
			return err
		}
	}
	err = remTags.RemoveFromPost(q, p)
	if err != nil {
		log.Print(err)
		return err
	}

	// p.setQ(nil)

	C.Cache.Purge("TC", strconv.Itoa(p.QID(DB)))

	return err
}

func (p *Post) EditTags(user *User, tagStrAdd, tagStrRem string) error {
	var addTags TagCollector
	err := addTags.Parse(tagStrAdd)
	if err != nil {
		//log.Print(err)
	}
	var remTags TagCollector
	err = remTags.Parse(tagStrRem)
	if err != nil {
		//log.Print(err)
	}

	if len(addTags.Tags) < 1 && len(remTags.Tags) < 1 {
		return errors.New("no tags in edit")
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	q := tx
	//p.setQ(tx)

	var at []*Tag
	for _, t := range addTags.Tags {
		a := NewAlias()
		a.Tag = t
		//a.setQ(tx)
		b := a.QTo(tx)
		if b.QID(tx) == 0 {
			b = t
		}

		if b.QID(tx) == 0 {
			err = b.Save(tx)
			if err != nil {
				log.Print(err)
				return txError(tx, err)
			}
		}

		var tmp int
		err = tx.QueryRow("SELECT count(*) FROM post_tag_mappings WHERE post_id=$1 AND tag_id=$2", p.QID(q), b.QID(q)).Scan(&tmp)
		if err == nil && tmp > 0 {
			continue // Tag is already on this post
		}

		at = append(at, b)

	}

	var rt []*Tag
	for _, t := range remTags.Tags {
		a := NewAlias()
		a.Tag = t
		b := a.QTo(tx)
		if b.QID(tx) == 0 {
			b = t
		}

		if b.QID(tx) == 0 {
			continue
		}

		var exist int
		err := tx.QueryRow("SELECT count(1) FROM post_tag_mappings WHERE post_id=$1 AND tag_id=$2", p.QID(q), b.QID(q)).Scan(&exist)
		if err != nil || exist == 0 {
			// Tag does not exist on this post
			continue
		}

		rt = append(rt, b)
	}

	if len(at) < 1 && len(rt) < 1 {
		return txError(tx, errors.New("no tags in edit"))
	}

	addTags.Tags = at
	remTags.Tags = rt
	var historyID int
	err = tx.QueryRow("INSERT INTO tag_history(user_id, post_id, timestamp) VALUES($1, $2, CURRENT_TIMESTAMP) RETURNING id", user.QID(q), p.QID(q)).Scan(&historyID)
	if err != nil {
		log.Println(err)
		return txError(tx, err)
	}

	for _, tag := range at {
		_, err = tx.Exec("INSERT INTO edited_tags(history_id, tag_id, direction) VALUES($1, $2, $3)", historyID, tag.QID(q), 1)
		if err != nil {
			log.Println(err)
			return txError(tx, err)
		}
	}
	addTags.AddToPost(tx, p)

	for _, tag := range rt {
		_, err = tx.Exec("INSERT INTO edited_tags(history_id, tag_id, direction) VALUES($1, $2, $3)", historyID, tag.QID(q), -1)
		if err != nil {
			return txError(tx, err)
		}
	}
	err = remTags.RemoveFromPost(tx, p)
	if err != nil {
		log.Print(err)
		return txError(tx, err)
	}

	err = tx.Commit()

	// p.setQ(nil)

	C.Cache.Purge("TC", strconv.Itoa(p.QID(DB)))

	return err
}

func (p *Post) FindSimilar(q querier, dist int) ([]*Post, error) {
	if p.QID(q) == 0 {
		return nil, errors.New("id = 0")
	}

	type phash struct {
		post_id int
		h1      uint16
		h2      uint16
		h3      uint16
		h4      uint16
	}

	var ph phash

	err := q.QueryRow("SELECT * FROM phash WHERE post_id = $1", p.QID(q)).Scan(&ph.post_id, &ph.h1, &ph.h2, &ph.h3, &ph.h4)
	if err != nil {
		return nil, err
	}

	rows, err := q.Query("SELECT * FROM phash WHERE h1=$1 OR h2=$2 OR h3=$3 OR h4=$4 ORDER BY post_id DESC", ph.h1, ph.h2, ph.h3, ph.h4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phs []phash

	for rows.Next() {
		var phn phash
		if err = rows.Scan(&phn.post_id, &phn.h1, &phn.h2, &phn.h3, &phn.h4); err != nil {
			return nil, err
		}
		phs = append(phs, phn)
	}
	f := func(a phash) imgsim.Hash {
		return imgsim.Hash(uint64(a.h1)<<16 | uint64(a.h2)<<32 | uint64(a.h3)<<48 | uint64(a.h4)<<64)
	}

	var posts []*Post
	hasha := f(ph)

	for _, h := range phs {
		hashb := f(h)
		if imgsim.Distance(hasha, hashb) < dist {
			pst := NewPost()
			pst.ID = h.post_id
			posts = append(posts, pst)
		}
	}

	return posts, nil
}

func (p *Post) Comics(q querier) []*Comic {
	if p.QID(q) == 0 {
		return nil
	}
	rows, err := q.Query("SELECT comic_id FROM comic_mappings WHERE post_id=$1", p.QID(q))
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

func (p *Post) Duplicate() *Duplicate {
	d := NewDuplicate()
	d.Post = p

	return d
}

func (p *Post) NewComment() *PostComment {
	pc := newPostComment()
	pc.Post = p
	return pc
}

func (p *Post) Comments(q querier) []*PostComment {
	if p.QID(q) <= 0 {
		return nil
	}

	rows, err := q.Query("SELECT id, user_id, text, timestamp FROM post_comments WHERE post_id = $1 ORDER BY id DESC", p.QID(q))
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

type PostCollector struct {
	posts    map[int][]*Post
	id       []int
	blackTag []int
	unless   []int

	tags       []*Tag //Sidebar
	TotalPosts int

	order string

	tagLock sync.Mutex
}

var perSlice = 500

func (pc *PostCollector) Get(tagString, blackTagString, unlessString, order string, limit, offset int) error {
	//var tagIDs []int

	in := func(i int, arr []int) bool {
		for _, j := range arr {
			if i == j {
				return true
			}
		}
		return false
	}
	if len(tagString) >= 1 {
		var tc TagCollector

		err := tc.Parse(tagString)
		if err != nil {
			return err
		}

		for _, tag := range tc.Tags {
			if tag.QID(DB) == 0 {
				// No posts will be available, return
				pc.id = []int{-1}
				return nil
			}
			alias := NewAlias()
			alias.Tag = tag
			if alias.QTo(DB).QID(DB) != 0 {
				tag = alias.QTo(DB)
			}
			pc.id = append(pc.id, tag.QID(DB))
		}
		sort.Ints(pc.id)
		//fmt.Println(tagIDs)
		if len(pc.id) <= 0 {
			pc.id = []int{-1}
		}
	}

	if len(blackTagString) >= 1 {
		var tc TagCollector

		err := tc.Parse(blackTagString)
		if err != nil {
			return err
		}

		for _, tag := range tc.Tags {
			if tag.QID(DB) == 0 {
				continue
			}

			alias := NewAlias()
			alias.Tag = tag
			if alias.QTo(DB).QID(DB) != 0 {
				tag = alias.QTo(DB)
			}

			// Cant have your tag and filter it too
			if in(tag.QID(DB), pc.id) {
				continue
			}

			pc.blackTag = append(pc.blackTag, tag.QID(DB))
		}
		sort.Ints(pc.blackTag)
	}
	//fmt.Println(tagIDs)
	//pc.id = tagIDs //idStr

	if len(unlessString) >= 1 {
		var tc TagCollector

		err := tc.Parse(unlessString)
		if err != nil {
			return err
		}

		for _, tag := range tc.Tags {
			if tag.QID(DB) == 0 {
				continue
			}

			alias := NewAlias()
			alias.Tag = tag
			if alias.QTo(DB).QID(DB) != 0 {
				tag = alias.QTo(DB)
			}

			// Cant filter your tag and include it too
			if in(tag.QID(DB), pc.blackTag) || in(tag.QID(DB), pc.id) {
				continue
			}

			pc.unless = append(pc.unless, tag.QID(DB))
		}
		sort.Ints(pc.unless)
	}

	switch strings.ToUpper(order) {
	case "ASC", "DESC":
		pc.order = strings.ToUpper(order)
	case "RANDOM":
		pc.order = strings.ToUpper(order) + "()"
	default:
		pc.order = "DESC"
	}

	if t := C.Cache.Get("PC", pc.idStr()); t != nil {
		tmp, ok := t.(*PostCollector)
		if ok {
			*pc = *tmp
			// pc.posts = tmp.posts
			// pc.id = tmp.ID
			// pc.TotalPosts = tmp.TotalPosts
			// pc.tags = tmp.tags
			// if ok2 := tmp.GetW(ulimit, uoffset); ok2 != nil {
			// 	return nil
			// }
		}
	} else {
		C.Cache.Set("PC", pc.idStr(), pc)
	}

	return pc.search(limit, offset)
}

func (pc *PostCollector) idStr() string {
	// if len(pc.id) <= 0 {
	// 	return "0"
	// }
	var str string
	if len(pc.id) >= 1 {
		for _, i := range pc.id {
			str = fmt.Sprint(str+" ", i)
		}
	} else {
		str = fmt.Sprint(str, " ", 0)
	}
	str += " -"
	for _, i := range pc.blackTag {
		str = fmt.Sprint(str+" ", i)
	}
	str += " -"
	for _, i := range pc.unless {
		str = fmt.Sprint(str+" ", i)
	}

	str += pc.order
	str = strings.TrimSpace(str)
	// fmt.Println("PCSTR", str)
	return str
}

func (pc *PostCollector) search(ulimit, uoffset int) error {

	if pc.idStr() == "-1" {
		return nil
	}

	rand := func(pre, order string) string {
		if order == "RANDOM()" {
			return order
		}
		return fmt.Sprintf(pre, order)
	}

	// emptyRand := func(pre, str string) string {
	// 	if str == "RANDOM()" {
	// 		return ""
	// 	}
	// 	return fmt.Sprintf(pre, str)
	// }

	//fmt.Println(rand("test %s", pc.order))

	//pc.id = tagString

	//pc.Posts2.Get(ulimit, uoffset)

	limit := perSlice
	offset := ((uoffset + ulimit) / limit) * limit
	//fmt.Println("Real Offset ", offset)
	var rows *sql.Rows
	var err error
	//fmt.Println(pc.idStr())

	if ok := pc.GetW(ulimit, uoffset); ok != nil {
		return nil
	}

	//fmt.Println("Cache not accessed")

	//pc.id = idStr
	//fmt.Println(idStr)
	if len(pc.id) > 0 {
		var innerStr string
		var endStr = "WHERE "
		var blt string
		if len(pc.blackTag) >= 1 {
			endStr += "("
			var or, un string
			for _, t := range pc.blackTag {
				or += fmt.Sprint(" f1.tag_id = ", t, " OR")
			}
			or = strings.TrimRight(or, " OR")

			for _, t := range pc.unless {
				un += fmt.Sprint(" u1.tag_id = ", t, " OR")
			}
			un = strings.TrimRight(un, " OR")
			if un != "" {
				un = fmt.Sprint(" LEFT OUTER JOIN post_tag_mappings u1 ON t1.post_id = u1.post_id AND(", un, ")")
				endStr += "u1.post_id IS NOT NULL OR "
			}

			blt = fmt.Sprint("FULL OUTER JOIN post_tag_mappings f1 ON t1.post_id = f1.post_id AND(", or, ") ", un)
			// fmt.Println(blt)
			endStr += "f1.post_id IS NULL) AND "
		}

		//innerStr = "SELECT DISTINCT t1.post_id FROM post_tag_mappings t1 "
		innerStr = "JOIN post_tag_mappings t1 ON p1.id = t1.post_id "

		if len(pc.id) > 1 {
			for i, tagID := range pc.id {
				var tstr string
				if i+1 == len(pc.id) {
					endStr += fmt.Sprintf("t1.tag_id = %d ", tagID)
				} else {
					tstr = fmt.Sprintf("JOIN post_tag_mappings t%d ON t%d.post_id = t%d.post_id ", i+2, i+1, i+2)
					endStr += fmt.Sprintf("t%d.tag_id = %d AND ", i+2, tagID)
				}
				innerStr += tstr
			}
		} else {
			endStr = fmt.Sprintf(endStr+"t1.tag_id = %d", pc.id[0])
		}
		innerStr += blt + endStr

		str := fmt.Sprintf("SELECT id, multihash, mime_id FROM posts p1 %s AND p1.deleted = false ORDER BY %s LIMIT $1 OFFSET $2", innerStr, rand("p1.id %s", pc.order))

		//fmt.Println(str)

		if pc.TotalPosts <= 0 {
			count := fmt.Sprintf("SELECT count(*) FROM posts p1 %s AND p1.deleted = false", innerStr)
			err = DB.QueryRow(count).Scan(&pc.TotalPosts)
			if err != nil {
				log.Print(err)
				return err
			}
		}

		rows, err = DB.Query(str, limit, offset)
		if err != nil {
			log.Print(err)
			return err
		}

	} else if len(pc.blackTag) > 0 {
		var innerStr, endStr string
		var blt string
		var or, un string

		endStr = "WHERE "
		for _, t := range pc.blackTag {
			or += fmt.Sprint(" f1.tag_id = ", t, " OR")
		}
		or = strings.TrimRight(or, " OR")

		for _, t := range pc.unless {
			un += fmt.Sprint(" u1.tag_id = ", t, " OR")
		}
		un = strings.TrimRight(un, " OR")
		if un != "" {
			un = fmt.Sprint(" LEFT OUTER JOIN post_tag_mappings u1 ON p1.id = u1.post_id AND(", un, ")")
			endStr += "(u1.post_id IS NOT NULL OR f1.post_id IS NULL) "
		} else {
			endStr += "f1.post_id IS NULL "
		}

		blt = fmt.Sprint("FULL OUTER JOIN post_tag_mappings f1 ON p1.id = f1.post_id AND(", or, ") ", un)

		// innerStr = "JOIN post_tag_mappings t1 ON p1.id = t1.post_id "
		innerStr += blt + endStr

		str := fmt.Sprintf("SELECT id, multihash, mime_id FROM posts p1 %s AND p1.deleted = false ORDER BY %s LIMIT $1 OFFSET $2", innerStr, rand("id %s", pc.order))

		//fmt.Println(str)

		if pc.TotalPosts <= 0 {
			count := fmt.Sprintf("SELECT count(*) FROM posts p1 %s AND p1.deleted = false", innerStr)
			err = DB.QueryRow(count).Scan(&pc.TotalPosts)
			if err != nil {
				log.Print(err)
				return err
			}
		}

		//fmt.Println(str)
		rows, err = DB.Query(str, limit, offset)
		if err != nil {
			log.Print(err)
			return err
		}
	} else {
		pc.TotalPosts = GetTotalPosts()

		var err error
		//query := fmt.Sprintf("SELECT id FROM posts ORDER BY id %s LIMIT $1 OFFSET $2", order)
		query := fmt.Sprintf("SELECT id, multihash, mime_id FROM posts WHERE deleted = false ORDER BY %s LIMIT $1 OFFSET $2", rand("id %s", pc.order))
		rows, err = DB.Query(query, limit, offset)
		if err != nil {
			return err
		}
	}
	defer rows.Close()

	var tmpPosts []*Post
	for rows.Next() {
		post := NewPost()
		err := rows.Scan(&post.ID, &post.Hash, &post.Mime.ID)

		if err != nil {
			return err
		}
		tmpPosts = append(tmpPosts, post)
	}

	pc.set((uoffset+ulimit)/limit, tmpPosts)
	//pc.Posts = pc.get(ulimit, uoffset)
	err = rows.Err()

	//C.Cache.Set("PC", pc.idStr(), pc)

	return err
}

func (pc *PostCollector) Tags(maxTags int) []*Tag {
	pc.tagLock.Lock()
	defer pc.tagLock.Unlock()
	if len(pc.tags) > 0 {
		return pc.tags
	}
	//fmt.Println(pc.id)
	var allTagIds [][]int

	if pc.idStr() == "-1" {
		return nil
	}

	// Get the first batch
	pc.search(10, 0)
	//pc.GetW(10, 0)

	// Get tags from all posts
	for _, post := range pc.posts[0] {
		var ptc TagCollector
		err := ptc.GetFromPost(DB, *post)
		if err != nil {
			continue
		}
		var ids []int
		for _, tag := range ptc.Tags {
			ids = append(ids, tag.QID(DB))
		}
		allTagIds = append(allTagIds, ids)
	}

	type tagMap struct {
		id    int
		count int
	}

	// Count all tags
	var tagIDArr []tagMap
	for _, idSet := range allTagIds {
		for _, id := range idSet {
			newID := true
			for i, id2 := range tagIDArr {
				if id == id2.id {
					tagIDArr[i].count++
					newID = false
					break
				}
			}
			if newID {
				tagIDArr = append(tagIDArr, tagMap{id, 1})
			}
		}
	}

	// Sort all tags
	swapped := true
	for swapped {
		swapped = false
		for i := 1; i < len(tagIDArr); i++ {
			if tagIDArr[i-1].count < tagIDArr[i].count {
				tmp := tagIDArr[i]
				tagIDArr[i] = tagIDArr[i-1]
				tagIDArr[i-1] = tmp
				swapped = true
			}
		}
	}

	// Hotfix for when cache is gc'd and there are multiple calls for this search
	pc.tags = nil
	// Retrive and append the tags
	arrLimit := maxTags
	if len(tagIDArr) < arrLimit {
		arrLimit = len(tagIDArr)
	}
	for i := 0; i < arrLimit; i++ {
		t := NewTag()
		t.SetID(tagIDArr[i].id)

		pc.tags = append(pc.tags, t)
		//p.Sidebar.Tags = append(p.Sidebar.Tags, tagstruct{Tag{t.Tag(), t.Namespace().Namespace()}, tagIDArr[i].count})
	}

	// err = tx.Commit()
	// if err != nil {
	// 	log.Print(err)
	// }
	// tx = nil

	C.Cache.Set("PC", pc.idStr(), pc)

	return pc.tags
}

var totalPosts int

func GetTotalPosts() int {
	if totalPosts != 0 {
		return totalPosts
	}
	err := DB.QueryRow("SELECT count(*) FROM posts WHERE deleted=false").Scan(&totalPosts)
	if err != nil {
		log.Println(err)
		return totalPosts
	}
	return totalPosts
}

func resetCacheTag(tagID int) {
	C.Cache.Purge("PC", strconv.Itoa(tagID))
	C.Cache.Purge("TAG", strconv.Itoa(tagID))
	C.Cache.Purge("PC", "0")
}

func (p *PostCollector) GetW(limit, offset int) []*Post {
	if p.posts == nil {
		p.posts = make(map[int][]*Post)
	}
	if limit <= 0 || offset < 0 {
		return nil
	}
	//fmt.Println("begOff", offset/perSlice) // beginning offset
	begOff := offset / perSlice

	//fmt.Println("endOff ", (offset+limit)/perSlice) // end offset
	endOff := (offset + limit) / perSlice

	//fmt.Println("first offset ", offset%perSlice)
	frstOff := offset % perSlice

	//fmt.Println("seccond offset ", (offset+limit)%perSlice)
	secOff := (offset + limit) % perSlice

	var posts = []*Post{}

	if begOff == endOff {
		//fmt.Println("Single")
		tmp, ok := p.posts[begOff]
		if !ok {
			//log.Print("FATAL ERROR")
			return nil
		}
		//fmt.Println(len(tmp), frstOff, secOff)
		if (len(tmp) - 1) < frstOff {
			return nil
		}
		//fmt.Println(frstOff, secOff, len(tmp))
		posts = append(posts, tmp[frstOff:max(len(tmp), secOff)]...)
	} else {
		//fmt.Println("Double")
		tmp, ok := p.posts[begOff]
		if !ok {
			//fmt.Println("Not ok1")
			p.search(limit, offset-limit)
			tmp, ok = p.posts[begOff]
			if !ok {
				//log.Print("Fatal erorr")
				return nil
			}
		}
		if tmp == nil {
			return nil
		}
		posts = append(posts, tmp[max(len(tmp)-1, frstOff):]...)

		tmp, ok = p.posts[endOff]
		if ok {
			if len(tmp) > 0 {
				posts = append(posts, tmp[:max(len(tmp), secOff)]...)
			}
		} else {
			//log.Print("FATAL error")
			return nil
		}
	}
	return posts
}

// Returns whichever is smaller
func Smal(x, y int) int {
	return max(x, y)
}

// Returns whichever is larger
func Larg(x, y int) int {
	return min(x, y)
}

// Return the smallest of the 2
func Max(x, y int) int {
	return max(x, y)
}

// Return the largest of the 2
func Min(x, y int) int {
	return min(x, y)
}

func max(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func min(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func (p *PostCollector) set(offset int, posts []*Post) {
	if p.posts == nil {
		p.posts = make(map[int][]*Post)
	}
	//fmt.Println("Setting Offset = ", offset)
	p.posts[offset] = posts
}
