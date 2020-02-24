package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
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

//var tagCacheLock sync.RWMutex

func CachedTag(t *Tag) *Tag {
	//tagCacheLock.RLock()
	if ct := C.Cache.Get("TAG", strconv.Itoa(t.ID)); ct != nil {
		//tagCacheLock.RUnlock()
		return ct.(*Tag)
	}
	//tagCacheLock.RUnlock()
	//tagCacheLock.Lock()
	C.Cache.Set("TAG", strconv.Itoa(t.ID), t)
	//tagCacheLock.Unlock()
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
	err := q.QueryRow("SELECT tag FROM tags WHERE tags.id=$1", t.ID).Scan(&t.Tag)
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

	if t.QID(q) != 0 && t.Namespace.ID == 0 {
		err := q.QueryRow("SELECT namespace_id FROM tags WHERE id=$1", t.ID).Scan(&t.Namespace.ID)
		if err != nil {
			log.Print(err)
		}
	}

	t.Namespace = CachedNamespace(t.Namespace)

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
		return errors.New(fmt.Sprintf("tag: not enough arguments. Expected Tag got '%s'", t.Tag))
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

// Returns the tag it has been aliased to or itself if none
func (t *Tag) aliasedTo(q querier) (*Tag, error) {
	var alias = NewAlias()
	alias.Tag = t

	to, err := alias.QTo(q)
	if err != nil {
		return nil, err
	}

	if to.ID == 0 {
		return t, nil
	}

	return to, nil
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

	child, err := t.aliasedTo(q)
	if err != nil {
		return err
	}
	aParent, err := parent.aliasedTo(q)
	if err != nil {
		return err
	}

	if _, err = q.Exec(`
			INSERT INTO post_tag_mappings(
				post_id, tag_id
			)
			SELECT post_id, $1
			FROM post_tag_mappings
			WHERE tag_id = $2
			ON CONFLICT DO NOTHING
		`,
		aParent.ID,
		child.ID,
	); err != nil {
		log.Println(err)
		return err
	}

	if _, err := q.Exec(`
			INSERT INTO parent_tags(
				parent_id, child_id
			)
			VALUES ($1, $2)
		`,
		aParent.ID,
		child.ID,
	); err != nil {
		log.Println(err)
		return err
	}

	resetCacheTag(aParent.ID)
	resetCacheTag(child.ID)
	return nil
}

func (t *Tag) allParents(q querier) []*Tag {
	var parents []*Tag

	// Recursivly resolve parents
	var process func(tags []*Tag)

	process = func(tags []*Tag) {
		for _, tag := range tags {
			if !isTagIn(tag, parents) {
				parents = append(parents, tag)
				process(tag.Parents(q))
			}
		}
	}

	process(t.Parents(q))

	return parents
}

func (t *Tag) Parents(q querier) []*Tag {
	if t.QID(q) <= 0 {
		return nil
	}

	rows, err := q.Query("SELECT parent_id FROM parent_tags WHERE child_id = $1", t.QID(q))
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println(err)
		}
		return nil
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		nt := NewTag()
		err = rows.Scan(&nt.ID)
		if err != nil {
			log.Println(err)
			return nil
		}
		tags = append(tags, nt)
	}

	if rows.Err() != nil {
		log.Println(err)
		return nil
	}

	return tags
}

func (t *Tag) Children(q querier) []*Tag {
	if t.QID(q) <= 0 {
		return nil
	}

	rows, err := q.Query("SELECT child_id FROM parent_tags WHERE parent_id = $1", t.QID(q))
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println(err)
		}
		return nil
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		nt := NewTag()
		err = rows.Scan(&nt.ID)
		if err != nil {
			log.Println(err)
			return nil
		}

		tags = append(tags, nt)
	}

	if rows.Err() != nil {
		log.Println(err)
		return nil
	}

	return tags
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
	dupe, err := getDupeFromPost(q, p)
	if err != nil {
		return err
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
			to, err := a.QTo(q)
			if err != nil {
				return err
			}

			if to.QID(q) != 0 {
				tag = to
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
			if _, err = q.Exec(execStr, i, dupe.Post.QID(q)); err != nil {
				log.Print(err)
			}
			resetCacheTag(i)
		}

		_, err = q.Exec(execStr, tag.QID(q), dupe.Post.QID(q))
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

	dupe, err := getDupeFromPost(q, p)
	if err != nil {
		return err
	}

	for _, t := range tc.Tags {

		if t.QID(q) == 0 {
			log.Print("TagCollector: RemoveFromPost: tag invalid", t)
			continue
		}

		a := NewAlias()
		// a.setQ(q)
		a.Tag = t
		to, err := a.QTo(q)
		if err != nil {
			return err
		}

		if to.QID(q) != 0 {
			t = to
		}

		resetCacheTag(t.QID(q))

		_, err = q.Exec("DELETE FROM post_tag_mappings WHERE tag_id=$1 AND post_id=$2", t.QID(q), dupe.Post.QID(q))

		if err != nil {
			log.Print(err)
			continue
		}
	}
	return nil
}

func (tc *TagCollector) Get(limit, offset int) error {
	rows, err := DB.Query("SELECT id, tag, namespace_id FROM tags ORDER BY id LIMIT $1 OFFSET $2", limit, offset)
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
		tag = CachedTag(tag)
		tag.Namespace = CachedNamespace(tag.Namespace)
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

func (tc *TagCollector) GetFromPost(q querier, p *Post) error {
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
		tag = CachedTag(tag)
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

	dupe, err := getDupeFromPost(q, p)
	if err != nil {
		return err
	}

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
		dupe.Post.QID(q))
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

	C.Cache.Set("TC", strconv.Itoa(dupe.Post.ID), tc)

	return rows.Err()
}

func (tc *TagCollector) save(tx querier) error {
	for _, tag := range tc.Tags {
		if err := tag.Save(tx); err != nil {
			return err
		}
	}

	return nil
}

func (tc *TagCollector) upgrade(q querier, upgradeParents bool) error {
	var newTags []*Tag

	for _, tag := range tc.Tags {
		a := NewAlias()
		a.Tag = tag
		b, err := a.QTo(q)
		if err != nil {
			return err
		}
		if b.ID != 0 {
			if !isTagIn(b, newTags) {
				newTags = append(newTags, b)
			}
		} else if !isTagIn(tag, newTags) {
			newTags = append(newTags, tag)
		}
	}

	if upgradeParents {
		for _, tag := range newTags {
			for _, parent := range tag.allParents(q) {
				if !isTagIn(parent, newTags) {
					newTags = append(newTags, parent)
				}
			}
		}
	}

	tc.Tags = newTags

	return nil
}

func (tc *TagCollector) SuggestedTags(q querier) TagCollector {
	var ntc TagCollector
	for _, tag := range tc.Tags {
		if c := C.Cache.Get("ST", tag.Namespace.Namespace+":"+tag.Tag); c != nil {
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
		if tag.Namespace.Namespace == "none" {
			rows, err = q.Query("SELECT id, tag, namespace_id FROM tags WHERE id IN(SELECT coalesce(alias_to, id) FROM tags LEFT JOIN alias ON id = alias_from WHERE tag LIKE($1)) ORDER BY count DESC LIMIT 10", "%"+strings.Replace(tag.Tag, "%", "\\%", -1)+"%")
		} else {
			rows, err = q.Query("SELECT id, tag, namespace_id FROM tags WHERE id IN(SELECT coalesce(alias_to, id) FROM tags LEFT JOIN alias ON id = alias_from WHERE namespace_id=$1 AND tag LIKE($2)) ORDER BY count DESC LIMIT 10", tag.Namespace.QID(q), "%"+strings.Replace(tag.Tag, "%", "\\%", -1)+"%")
		}
		if err != nil {
			log.Print(err)
			continue
		}
		var newTags []*Tag
		for rows.Next() {
			t := NewTag()
			//var cnt int
			err = rows.Scan(&t.ID, &t.Tag, &t.Namespace.ID)
			if err != nil {
				log.Print(err)
				break
			}
			t = CachedTag(t)
			t.Namespace = CachedNamespace(t.Namespace)
			//fmt.Println(cnt)
			newTags = append(newTags, t)
		}
		rows.Close()
		C.Cache.Set("ST", tag.Namespace.Namespace+":"+tag.Tag, newTags)
		ntc.Tags = append(ntc.Tags, newTags...)
	}
	return ntc
}

func isTagIn(a *Tag, tags []*Tag) bool {
	for _, b := range tags {
		if a.ID == b.ID {
			return true
		}
	}

	return false
}
