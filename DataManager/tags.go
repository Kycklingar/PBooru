package DataManager

import (
	"database/sql"
	"errors"
	"log"
	"strconv"
	"strings"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

func NewTag() *Tag {
	var t Tag
	t.Namespace = NewNamespace()
	t.Count = -1
	return &t
}

type Tag struct {
	ID        int
	Tag       string
	Namespace *Namespace
	Count     int
}

func CachedTag(t *Tag) *Tag {
	if ct := C.Cache.Get("TAG", strconv.Itoa(t.ID)); ct != nil {
		return ct.(*Tag)
	}
	C.Cache.Set("TAG", strconv.Itoa(t.ID), t)
	return t
}

func (t *Tag) QID(q querier) int {
	if t.ID != 0 {
		return t.ID
	}

	if t.Tag == "" {
		return 0
	}
	if t.Namespace.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT id FROM tags WHERE tag=$1 AND namespace_id=$2", t.Tag, t.Namespace.QID(q)).Scan(&t.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return 0
	}
	return t.ID
}

func (t *Tag) SetID(id int) {
	t.ID = id
}

func (t *Tag) QTag(q querier) string {
	if t.Tag != "" {
		return t.Tag
	}
	if t.ID == 0 {
		return ""
	}
	err := q.QueryRow("SELECT tag, nspace FROM tags JOIN namespaces ON namespaces.id = tags.namespace_id WHERE tags.id=$1", t.ID).Scan(&t.Tag, &t.Namespace.Namespace)
	if err != nil {
		log.Print(err)
		return ""
	}
	return t.Tag
}

func (t *Tag) QNamespace(q querier) *Namespace {
	if t.Namespace.QID(q) != 0 {
		return t.Namespace
	}
	if t.QID(q) != 0 {
		err := q.QueryRow("SELECT namespace_id FROM tags WHERE id=$1", t.ID).Scan(&t.Namespace.ID)
		if err != nil {
			log.Print(err)
		}
	}

	return t.Namespace
}

func (t *Tag) QCount(q querier) int {
	if t.Count > -1 {
		return t.Count
	}

	if t.QID(q) == 0 {
		return 0
	}

	err := q.QueryRow("SELECT count FROM tags WHERE id=$1", t.ID).Scan(&t.Count)
	if err != nil {
		log.Print(err)
		return 0
	}

	return t.Count
}

func (t *Tag) Save(q querier) error {
	if t.QID(q) != 0 {
		return nil
	}

	if t.Tag == "" {
		return errors.New("tag: not enough arguments")
	}

	if t.Namespace.QID(q) == 0 {
		err := t.Namespace.Save(q)
		if err != nil {
			return err
		}
	}

	if strings.ContainsAny(t.Tag, ",") {
		return errors.New("Tag cannot contain ','")
	}

	err := q.QueryRow("INSERT INTO tags(tag, namespace_id) VALUES($1, $2) RETURNING id", t.Tag, t.Namespace.QID(q)).Scan(&t.ID)
	if err != nil {
		return err
	}

	return nil
}

func (t *Tag) Parse(tagStr string) error {
	strSlc := strings.SplitN(strings.ToLower(strings.TrimSpace(tagStr)), ":", 2)

	// if len(strSlc[0]) <= 0 {
	// 	err := errors.New("no tag")
	// 	return t, err
	// }

	// Messy
	for i := range strSlc {
		strSlc[i] = strings.TrimSpace(strSlc[i])
	}
	if len(strSlc) == 1 && strSlc[0] != "" {
		t.Tag = strSlc[0]
		t.Namespace.Namespace = "none"
	} else if len(strSlc) == 2 && strSlc[0] != "" && strSlc[1] != "" {
		t.Tag = strSlc[1]
		t.Namespace.Namespace = strSlc[0]
	} else if len(strSlc) == 2 && (strSlc[1] != "") {
		t.Tag = strSlc[1]
		t.Namespace.Namespace = "none"
	} else {
		return errors.New("error decoding tag-string")
	}
	return nil
}

func (t *Tag) AddParent(q querier, parent *Tag) error {
	var err error
	if err = parent.Save(q); err != nil {
		log.Println(err)
		return err
	}
	if err = t.Save(q); err != nil {
		log.Println(err)
		return err
	}

	var parentID, childID int

	at := NewAlias()
	at.Tag = t
	if childID = at.QTo(q).QID(q); childID == 0 {
		childID = t.QID(q)
	}

	ap := NewAlias()
	ap.Tag = parent
	if parentID = ap.QTo(q).QID(q); parentID == 0 {
		parentID = parent.QID(q)
	}

	if _, err = q.Exec("INSERT INTO post_tag_mappings(post_id, tag_id) SELECT post_id, $1 FROM post_tag_mappings WHERE tag_id=$2 ON CONFLICT DO NOTHING", parentID, childID); err != nil {
		log.Println(err)
		return err
	}
	//fmt.Println(res.RowsAffected())

	if _, err := q.Exec("INSERT INTO parent_tags(parent_id, child_id) VALUES($1, $2)", parentID, childID); err != nil {
		log.Println(err)
		return err
	}
	resetCacheTag(parentID)
	resetCacheTag(childID)
	return nil
}

type TagCollector struct {
	Tags []*Tag
}

func (tc *TagCollector) Parse(tagStr string) error {
	tags := strings.Split(strings.Replace(tagStr, "\n", ",", -1), ",")
	for _, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			continue
		}
		t := NewTag()
		err := t.Parse(tag)
		if err != nil {
			//log.Print(fmt.Sprintf("error decoding tag: %s ", strings.TrimSpace(tag)), err)
			continue
		}
		tc.Tags = append(tc.Tags, t)
	}
	var err error
	// fmt.Println(t)
	if len(tc.Tags) < 1 {
		err = errors.New("error decoding any tags")
	}
	return err
}

func (tc *TagCollector) AddToPost(q querier, p *Post) error {
	var post = p

	d := NewDuplicate()
	d.Post = p
	if d.BestPost(q).QID(q) != 0 {
		post = d.BestPost(q)
	}

	for _, tag := range tc.Tags {
		if tag.QID(q) == 0 {
			err := tag.Save(q)
			if err != nil {
				return err
			}
		} else {
			a := NewAlias()
			a.Tag = tag
			if a.QTo(q).QID(q) != 0 {
				tag = a.QTo(q)
			}
		}

		var parentIDs []int
		rows, err := q.Query("SELECT parent_id FROM parent_tags WHERE child_id = $1", tag.QID(q))
		if err != nil {
			log.Print(err)
		}
		for rows.Next() {
			pid := 0
			if err = rows.Scan(&pid); err != nil {
				log.Println(err)
			}
			if pid == 0 {
				continue
			}
			parentIDs = append(parentIDs, pid)
		}
		rows.Close()

		execStr := "INSERT INTO post_tag_mappings VALUES($1, $2) ON CONFLICT DO NOTHING"

		for _, i := range parentIDs {
			if _, err = q.Exec(execStr, i, post.QID(q)); err != nil {
				log.Print(err)
			}
			resetCacheTag(i)
		}

		_, err = q.Exec(execStr, tag.QID(q), post.QID(q))
		if err != nil {
			log.Print(err)
			//return err
			//continue
		}
		resetCacheTag(tag.QID(q))
	}

	return nil
}

func (tc *TagCollector) RemoveFromPost(q querier, p *Post) error {
	post := p

	d := NewDuplicate()
	//d.setQ(q)
	d.Post = p
	if d.BestPost(q).QID(q) != 0 {
		post = p
	}

	for _, t := range tc.Tags {

		if t.QID(q) == 0 {
			log.Print("TagCollector: RemoveFromPost: tag invalid", t)
			continue
		}

		a := NewAlias()
		// a.setQ(q)
		a.Tag = t
		if a.QTo(q).QID(q) != 0 {
			t = a.QTo(q)
		}

		resetCacheTag(t.QID(q))

		_, err := q.Exec("DELETE FROM post_tag_mappings WHERE tag_id=$1 AND post_id=$2", t.QID(q), post.QID(q))
		if err != nil {
			log.Print(err)
			continue
		}
	}
	return nil
}

func (tc *TagCollector) Get(limit, offset int) error {
	rows, err := DB.Query("SELECT id, tag, namespace_id FROM tags LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tag := NewTag()
		err := rows.Scan(&tag.ID, &tag.Tag, &tag.Namespace.ID)
		if err != nil {
			return err
		}
		tc.Tags = append(tc.Tags, tag)
	}
	err = rows.Err()
	return err
}

func (tc *TagCollector) Total() int {
	var total int
	err := DB.QueryRow("SELECT count(*) FROM tags").Scan(&total)
	if err != nil {
		log.Println(err)
	}
	return total
}

func (tc *TagCollector) GetFromPost(q querier, p Post) error {
	if p.QID(q) == 0 {
		return errors.New("post invalid")
	}

	rows, err := q.Query("SELECT tag_id FROM post_tag_mappings WHERE post_id=$1", p.QID(q))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tag := NewTag()
		err = rows.Scan(&tag.ID)
		if err != nil {
			return err
		}
		tc.Tags = append(tc.Tags, tag)
	}
	return rows.Err()
}

func (tc *TagCollector) GetPostTags(q querier, p *Post) error {
	if p.QID(q) == 0 {
		return errors.New("post invalid")
	}

	if m := C.Cache.Get("TC", strconv.Itoa(p.QID(q))); m != nil {
		switch mm := m.(type) {
		case *TagCollector:
			*tc = *mm
			return nil
		}
	}

	dup := NewDuplicate()
	dup.Post = p
	//dup.setQ(q)
	p = dup.BestPost(q)
	// b := dup.BestPost()
	// if b.QID() != p.QID() {
	// 	p = *b
	// }

	rows, err := q.Query(
		`SELECT tags.id, tag, count, namespaces.id, nspace 
		FROM tags 
		JOIN namespaces 
		ON tags.namespace_id = namespaces.id 
		WHERE tags.id 
		IN(
			SELECT tag_id 
			FROM post_tag_mappings 
			WHERE post_id=$1
			)`,
		//ORDER BY nspace, tag`,
		p.QID(q))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tag := NewTag()
		err = rows.Scan(&tag.ID, &tag.Tag, &tag.Count, &tag.Namespace.ID, &tag.Namespace.Namespace)
		if err != nil {
			return err
		}
		tc.Tags = append(tc.Tags, tag)
	}

	// Sort in a pleasing way
	for i := 0; i < len(tc.Tags); i++ {
		for j := len(tc.Tags) - 1; j > i; j-- {
			tag1 := tc.Tags[i].QNamespace(q).QNamespace(q) + ":" + tc.Tags[i].QTag(q)
			tag2 := tc.Tags[j].QNamespace(q).QNamespace(q) + ":" + tc.Tags[j].QTag(q)
			if tc.Tags[i].QNamespace(q).QNamespace(q) == "none" {
				tag1 = tc.Tags[i].QTag(q)
			}
			if tc.Tags[j].QNamespace(q).QNamespace(q) == "none" {
				tag2 = tc.Tags[j].QTag(q)
			}

			if strings.Compare(tag1, tag2) == 1 {
				tmp := tc.Tags[i]
				tc.Tags[i] = tc.Tags[j]
				tc.Tags[j] = tmp
			}
		}
	}

	C.Cache.Set("TC", strconv.Itoa(p.QID(q)), tc)

	return rows.Err()
}

func (tc *TagCollector) SuggestedTags(q querier) TagCollector {
	var ntc TagCollector
	for _, tag := range tc.Tags {
		if c := C.Cache.Get("ST", tag.QNamespace(q).QNamespace(q)+":"+tag.QTag(q)); c != nil {
			//fmt.Println("Cache ST")
			switch m := c.(type) {
			case []*Tag:
				ntc.Tags = append(ntc.Tags, m...)
				continue
			default:
				log.Print("cache is not typeof []*Tag")
			}
		}

		var rows *sql.Rows
		var err error
		if tag.QNamespace(q).QNamespace(q) == "none" {
			rows, err = q.Query("SELECT tag, namespace_id FROM tags WHERE tag LIKE($1) ORDER BY count DESC LIMIT 10", "%"+strings.Replace(tag.QTag(q), "%", "\\%", -1)+"%")
		} else {
			rows, err = q.Query("SELECT tag, namespace_id FROM tags WHERE namespace_id=$1 AND tag LIKE($2) ORDER BY count DESC LIMIT 10", tag.QNamespace(q).QID(q), "%"+strings.Replace(tag.QTag(q), "%", "\\%", -1)+"%")
		}
		if err != nil {
			log.Print(err)
			continue
		}
		var newTags []*Tag
		for rows.Next() {
			t := NewTag()
			//var cnt int
			err = rows.Scan(&t.Tag, &t.Namespace.ID)
			if err != nil {
				log.Print(err)
				break
			}
			//fmt.Println(cnt)
			newTags = append(newTags, t)
		}
		rows.Close()
		C.Cache.Set("ST", tag.QNamespace(q).QNamespace(q)+":"+tag.QTag(q), newTags)
		ntc.Tags = append(ntc.Tags, newTags...)
	}
	return ntc
}
