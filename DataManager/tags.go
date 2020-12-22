package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/querier"
	"github.com/kycklingar/PBooru/DataManager/sqlbinder"
)

func NewTag() *Tag {
	var t Tag
	t.Namespace = NewNamespace()
	t.Count = -1
	return &t
}

func CachedTag(t *Tag) *Tag {
	if ct := C.Cache.Get("TAG", strconv.Itoa(t.ID)); ct != nil {
		return ct.(*Tag)
	}
	C.Cache.Set("TAG", strconv.Itoa(t.ID), t)
	return t
}

type Tag struct {
	ID        int
	Tag       string
	Namespace *Namespace
	Count     int
}

func (t *Tag) Rebind(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case FID:
		sel.Rebind(&t.ID)
	case FTag:
		sel.Rebind(&t.Tag)
	case FCount:
		sel.Rebind(&t.Count)
	case FNamespace:
		sel.Rebind(&t.Namespace.Namespace)
	}
}

func (t *Tag) String() string {
	if t.Namespace.Namespace == "none" {
		return t.Tag
	}

	return fmt.Sprint(t.Namespace.Namespace, ":", t.Tag)
}

func (t *Tag) EString() string {
	if t.Namespace.Namespace == "none" && strings.HasPrefix(t.Tag, ":") {
		return ":" + t.Tag
	}

	return t.String()
}

func (t *Tag) QID(q querier.Q) int {
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

func (t *Tag) QueryAll(q querier.Q) error {
	if t.ID <= 0 {
		return errors.New("No identifier")
	}

	if t.Tag != "" && t.Namespace.ID > 0 && t.Namespace.Namespace != "" {
		return nil
	}

	err := q.QueryRow(`
		SELECT tag, count, namespaces.id, nspace 
		FROM tags 
		JOIN namespaces 
		ON tags.namespace_id = namespaces.id 
		WHERE tags.id = $1
		`,
		t.ID,
	).Scan(&t.Tag, &t.Count, &t.Namespace.ID, &t.Namespace.Namespace)

	return err
}

func (t *Tag) SetID(id int) {
	t.ID = id
}

func (t *Tag) QTag(q querier.Q) string {
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

func (t *Tag) QNamespace(q querier.Q) *Namespace {
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

func (t *Tag) QCount(q querier.Q) int {
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

func (t *Tag) Save(q querier.Q) error {
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

func (t *Tag) recount(q querier.Q) error {
	_, err := q.Exec(`
		UPDATE tags
		SET count = (
			SELECT count(1)
			FROM post_tag_mappings
			WHERE tag_id = $1
		)
		WHERE id = $1
		`,
		t.ID,
	)
	return err
}

func (t *Tag) updateCount(q querier.Q, count int) error {
	_, err := q.Exec(`
		UPDATE tags
		SET count = count + $1
		WHERE id = $2
		`,
		count,
		t.ID,
	)
	return err
}

// Returns the tag it has been aliased to or itself if none
func (t *Tag) aliasedTo(q querier.Q) (*Tag, error) {
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

func (t *Tag) AddParent(q querier.Q, parent *Tag) error {
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

	err = aParent.recount(q)
	if err != nil {
		return err
	}

	err = child.recount(q)
	if err != nil {
		return err
	}

	resetCacheTag(q, aParent.ID)
	resetCacheTag(q, child.ID)
	return nil
}

func (t *Tag) allParents(q querier.Q) []*Tag {
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

func (t *Tag) Parents(q querier.Q) []*Tag {
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

func (t *Tag) Children(q querier.Q) []*Tag {
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

func (tc *TagCollector) BindField(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case FID:
		sel.Bind(nil, "t.id", "")
	case FTag:
		sel.Bind(nil, "t.tag", "")
	case FCount:
		sel.Bind(nil, "t.count", "")
	case FNamespace:
		sel.Bind(nil, "n.nspace", "JOIN namespaces n ON t.namespace_id = n.id")
	}
}

const (
	FID sqlbinder.Field = iota
	FTag
	FCount
	FNamespace
)

func (tc *TagCollector) FromPostMul(q querier.Q, p *Post, fields ...sqlbinder.Field) error {
	selector := sqlbinder.BindFieldAddresses(tc, fields...)

	query := fmt.Sprintf(`
		SELECT %s
		FROM post_tag_mappings ptm
		JOIN tags t ON tag_id = t.id
		%s
		WHERE post_id = $1
		`,
		selector.Select(),
		selector.Joins(),
	)

	rows, err := q.Query(query, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t = NewTag()
		selector.ReBind(t, fields...)

		err = rows.Scan(selector.Values()...)
		if err != nil {
			return err
		}

		tc.Tags = append(tc.Tags, t)
	}

	return nil
}

func (tc *TagCollector) GetFromPost(q querier.Q, p *Post) error {
	if p.ID == 0 {
		return errors.New("post invalid")
	}

	rows, err := q.Query("SELECT tag_id FROM post_tag_mappings WHERE post_id=$1", p.ID)
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

func (tc *TagCollector) GetPostTags(q querier.Q, p *Post) error {
	if p.ID == 0 {
		return errors.New("post invalid")
	}

	if m := C.Cache.Get("TPC", strconv.Itoa(p.ID)); m != nil {
		switch mm := m.(type) {
		case *TagCollector:
			*tc = *mm
			return nil
		}
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
		p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tag := NewTag()
		c := NewTag()
		err = rows.Scan(&c.ID, &tag.Tag, &tag.Count, &tag.Namespace.ID, &tag.Namespace.Namespace)
		if err != nil {
			return err
		}

		tag.ID = c.ID

		c = CachedTag(c)
		c = tag

		tc.Tags = append(tc.Tags, c)
	}

	C.Cache.Set("TPC", strconv.Itoa(p.ID), tc)

	return rows.Err()
}

func (tc *TagCollector) save(tx querier.Q) error {
	for _, tag := range tc.Tags {
		if err := tag.Save(tx); err != nil {
			return err
		}
	}

	return nil
}

func (tc *TagCollector) upgrade(q querier.Q, upgradeParents bool) error {
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

func (tc *TagCollector) SuggestedTags(q querier.Q) TagCollector {
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
			rows, err = q.Query(`
				SELECT id, tag, namespace_id
				FROM tags
				WHERE id IN(
					SELECT coalesce(alias_to, id)
					FROM tags
					LEFT JOIN alias
					ON id = alias_from
					WHERE tag LIKE('%'||$1||'%')
				)
				ORDER BY count DESC
				LIMIT 10`,
				//strings.Replace(tag.Tag, "%", "\\%", -1),
				tag.Tag,
			)
		} else {
			rows, err = q.Query(`
				SELECT id, tag, namespace_id
				FROM tags
				WHERE id IN(
					SELECT coalesce(alias_to, id)
					FROM tags
					LEFT JOIN alias
					ON id = alias_from
					WHERE namespace_id=$1
					AND tag LIKE('%'||$2||'%')
				)
				ORDER BY count DESC
				LIMIT 10
				`,
				tag.Namespace.QID(q),
				//strings.Replace(tag.Tag, "%", "\\%", -1),
				tag.Tag,
			)
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
