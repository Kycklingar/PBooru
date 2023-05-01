package DataManager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"

	mm "github.com/kycklingar/MinMax"
	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/postSearch"
	"github.com/kycklingar/PBooru/DataManager/search"
	"github.com/kycklingar/set"
)

func NewPostCollector() *PostSearch {
	return &PostSearch{TotalPosts: -1}
}

type SearchResult []resultSet

type resultSet struct {
	Post *Post
	Tags []Tag
}

type ErrorTag string

func (e ErrorTag) Error() string {
	return "tag does not exist: " + string(e)
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

type PostSearch struct {
	emptySet bool

	posts     map[int][]*Post
	and       set.Sorted[Tag]
	or        set.Sorted[Tag]
	filter    set.Sorted[Tag]
	unless    set.Sorted[Tag]
	tombstone bool

	tags       []Tag //Sidebar
	TotalPosts int

	mimeIDs []int
	order   string

	altGroup    int
	collectAlts bool

	tagLock sync.Mutex
	pl      sync.RWMutex
}

func CachedPostCollector(pc *PostSearch) *PostSearch {
	c := C.Cache.Get("PC", pc.idStr())
	if c != nil {
		return c.(*PostSearch)
	}

	C.Cache.Set("PC", pc.idStr(), pc)
	return pc
}

func (pc *PostSearch) SetOptions(opts SearchOptions) error {
	var err error
	pc.and, err = tsChain(parseTags(opts.And)).qids(DB).aliases(DB).unwrap()
	if err != nil {
		pc.emptySet = true
	}
	pc.or, err = tsChain(parseTags(opts.Or)).qids(DB).aliases(DB).unwrap()
	if err != nil {
		pc.emptySet = true
	}
	pc.filter, err = tsChain(parseTags(opts.Filter)).qids(DB).aliases(DB).unwrap()
	if err != nil {
		pc.emptySet = true
	}
	pc.unless, err = tsChain(parseTags(opts.Unless)).qids(DB).aliases(DB).unwrap()
	if err != nil {
		pc.emptySet = true
	}

	pc.order = strings.ToLower(opts.Order)
	pc.collectAlts = opts.AltCollect
	pc.altGroup = opts.Altgroup
	pc.tombstone = opts.Tombstone

	pc.mimeIDs = set.NewOrdered(opts.MimeIDs...).Slice()

	return nil
}

func (pc *PostSearch) countIDStr() string {
	if pc.emptySet {
		return "NIL"
	}

	if !pc.tombstone &&
		len(pc.and) <= 0 &&
		len(pc.or) <= 0 &&
		len(pc.filter) <= 0 &&
		len(pc.mimeIDs) <= 0 &&
		!pc.collectAlts &&
		pc.altGroup <= 0 {
		return "0"
	}

	return fmt.Sprint(
		tSetStr(pc.and),
		" - ",
		tSetStr(pc.or),
		" - ",
		tSetStr(pc.filter),
		" - ",
		tSetStr(pc.unless),
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

func (pc *PostSearch) idStr() string {
	if pc.emptySet {
		return "NIL"
	}

	return fmt.Sprint(
		pc.countIDStr(),
		" - ",
		pc.order,
	)
}
func (pc *PostSearch) Search(ctx context.Context, limit, offset int) (SearchResult, error) {
	if pc.emptySet {
		pc.TotalPosts = 0
		return nil, nil
	}

	var p = postSearch.NewPostsSearch("p")

	convertToIds := func(tags set.Sorted[Tag]) []int {
		var ret = make([]int, len(tags))
		for i := 0; i < len(tags); i++ {
			ret[i] = tags[i].ID
		}
		return ret
	}

	if len(pc.and) > 0 {
		for i, tag := range pc.and {
			var and = postSearch.NewPtm(fmt.Sprint("a", i), tag.ID)
			p.Join(&and).On(
				search.JoinCondition(
					p.Field("id"),
					and.Field("post_id"),
				),
			)
			p.Where(&and)
		}
	}

	if len(pc.or) > 0 {
		var or = postSearch.NewPtm("o", convertToIds(pc.or)...)
		p.Join(&or).On(
			search.JoinCondition(
				p.Field("id"),
				or.Field("post_id"),
			),
		)
		p.Where(&or)
	}

	if len(pc.filter) > 0 {
		var filter = postSearch.NewPtm("f", convertToIds(pc.filter)...)
		p.LeftJoin(&filter).On(
			search.JoinCondition(
				p.Field("id"),
				filter.Field("post_id"),
			),
		).And(&filter)

		var cond = search.IsNull(filter.Field("post_id"))

		if len(pc.unless) > 0 {
			var unless = postSearch.NewPtm("u", convertToIds(pc.unless)...)
			p.LeftJoin(&unless).On(
				search.JoinCondition(
					filter.Field("post_id"),
					unless.Field("post_id"),
				),
			).And(&unless)
			var group = search.Group(cond)
			group.Or(search.IsNotNull(unless.Field("post_id")))

			p.Where(group)
		} else {
			p.Where(cond)
		}
	}

	if pc.tombstone {
		var tombstone = search.NewTable("tombstone", "tombstone")
		p.LeftJoin(tombstone).On(
			search.JoinCondition(
				p.Field("id"),
				tombstone.Field("post_id"),
			),
		)

		p.Where(search.IsNotNull(tombstone.Field("post_id")))
	}

	if pc.altGroup > 0 {
		p.Where(postSearch.AltGroup(p.Field("alt_group"), pc.altGroup))
	}

	if len(pc.mimeIDs) > 0 {
		p.Where(postSearch.Mimes(p.Field("mime_id"), pc.mimeIDs))
	}

	p.Where(search.Boolean(p.Field("removed"), false))

	var order = search.NewOrder()
	var selection string
	var groupby string
	switch pc.order {
	case "asc":
		selection = "DISTINCT ON(p.id) p.id"
		order.By(p.Field("id"), search.Ascending)
	case "random":
		selection = "p.id"
		groupby = "\nGROUP BY p.id"
		order.By(p.Field("id"), search.Random)
		offset = 0
	case "score":
		selection = "DISTINCT ON(p.id, p.score) p.id"
		order.By(p.Field("score"), search.Descending).
			By(p.Field("id"), search.Descending)
	default:
		selection = "DISTINCT ON(p.id) p.id"
		order.By(p.Field("id"), search.Descending)
	}

	var query strings.Builder

	// Count
	if pc.TotalPosts < 0 {
		if pc.countIDStr() != "0" {
			c := pc.ccGet()
			if c < 0 {
				query.WriteString("SELECT count(DISTINCT ")
				if pc.collectAlts {
					query.WriteString(string(p.Field("alt_group")))
				} else {
					query.WriteString(string(p.Field("id")))
				}
				query.WriteString(")\n FROM ")
				p.Eval(&query)
				err := db.Context.QueryRow(query.String()).Scan(&pc.TotalPosts)
				if err != nil {
					return nil, err
				}
				pc.ccSet(pc.TotalPosts)
			} else {
				pc.TotalPosts = c
			}
		} else {
			pc.TotalPosts = GetTotalPosts()
		}
		query.Reset()
	}

	// No more posts beyond this point
	if pc.TotalPosts <= 0 || offset > pc.TotalPosts {
		return nil, nil
	}

	// Query
	query.WriteString("WITH a AS(")
	query.WriteString("\nSELECT ")
	query.WriteString(string(p.Field("*")))
	query.WriteString("\nFROM ")
	p.Eval(&query)
	query.WriteString("), b AS(")
	query.WriteString("\nSELECT ")
	query.WriteString(selection)
	query.WriteString("\nFROM ")
	if pc.collectAlts {
		query.WriteString("(")
		query.WriteString("\nSELECT DISTINCT ON(a.alt_group) a.*")
		query.WriteString("\nFROM a")
		query.WriteString("\nORDER BY a.alt_group, a.score DESC, a.id DESC")
		query.WriteString("\n) p")
	} else {
		query.WriteString("a p")
	}
	query.WriteString(groupby)
	order.Eval(&query)
	query.WriteString("\nLIMIT $1 OFFSET $2\n)")
	query.WriteString(`
		SELECT p.id, ptm.tag_id, t.tag, t.count, t.namespace
		FROM posts p
		LEFT JOIN post_tag_mappings ptm
		JOIN tag t
		ON t.id = ptm.tag_id
		ON p.id = ptm.post_id
		JOIN b
		ON b.id = p.id`)
	order.Eval(&query)

	// Collecting
	var (
		result    SearchResult
		prev      int
		resultMap = make(map[int]resultSet)
	)

	seqScan := func(scan db.Scanner) error {
		var (
			tagID     sql.NullInt64
			tagCount  sql.NullInt64
			tagName   sql.NullString
			namespace sql.NullString
			postID    int
		)

		err := scan(&postID, &tagID, &tagName, &tagCount, &namespace)
		if err != nil {
			return err
		}

		if prev != postID {
			p := NewPost()
			p.ID = postID
			result = append(result, resultSet{Post: p})
			prev = postID
		}

		if tagID.Valid {
			var t = Tag{
				ID:        int(tagID.Int64),
				Tag:       tagName.String,
				Namespace: Namespace(namespace.String),
				Count:     int(tagCount.Int64),
			}
			result[len(result)-1].Tags = append(result[len(result)-1].Tags, t)
		}

		return nil
	}

	randScan := func(scan db.Scanner) error {
		var (
			tagID     sql.NullInt64
			tagCount  sql.NullInt64
			tagName   sql.NullString
			namespace sql.NullString
			postID    int
		)

		err := scan(&postID, &tagID, &tagName, &tagCount, &namespace)
		if err != nil {
			return err
		}

		set, ok := resultMap[postID]
		if !ok {
			post := NewPost()
			post.ID = postID
			set = resultSet{Post: post}
		}

		if tagID.Valid {
			var t = Tag{
				ID:        int(tagID.Int64),
				Tag:       tagName.String,
				Namespace: Namespace(namespace.String),
				Count:     int(tagCount.Int64),
			}
			set.Tags = append(set.Tags, t)
		}

		resultMap[postID] = set

		return nil
	}

	var scan db.RowScanner = seqScan

	if pc.order == "random" {
		scan = randScan
	}

	err := db.QueryRowsContext(
		ctx,
		db.Context,
		query.String(),
		limit,
		offset,
	)(scan)

	if len(resultMap) > 0 {
		result = make(SearchResult, len(resultMap))
		var i int
		for _, set := range resultMap {
			result[i] = set
			i++
		}
	}

	return result, err
}

func (pc *PostSearch) Tags(ctx context.Context, maxTags int) []Tag {
	pc.tagLock.Lock()
	defer pc.tagLock.Unlock()
	if len(pc.tags) > 0 {
		return pc.tags
	}

	var allTags []Tag

	if pc.countIDStr() == "-1" {
		return nil
	}

	// Get the first batch
	result, err := pc.Search(ctx, 500, 0)
	if err != nil {
		log.Println(err)
		return nil
	}

	// Get tags from all posts
	pc.pl.RLock()
	for _, set := range result {
		allTags = append(allTags, set.Tags...)
	}
	pc.pl.RUnlock()

	type tagMap struct {
		tag   Tag
		count int
	}

	tm := make(map[int]tagMap)

	for _, tag := range allTags {
		m := tm[tag.ID]
		m.tag = tag
		m.count++
		tm[tag.ID] = m
	}

	var countMap = make([]tagMap, len(tm))
	var i int
	for _, v := range tm {
		countMap[i] = v
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

func (pc *PostSearch) ccGet() (c int) {
	if err := DB.QueryRow("SELECT count FROM search_count_cache WHERE str = $1", pc.countIDStr()).Scan(&c); err != nil {
		return -1
	}
	return
}

func (pc *PostSearch) ccSet(c int) {
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

	union := set.Union(pc.and, pc.or)
	union = set.Union(union, pc.filter)
	union = set.Union(union, pc.unless)

	for _, t := range union {
		tx.Exec("INSERT INTO search_count_cache_tag_mapping (cache_id, tag_id) VALUES($1, $2)", cid, t.ID)
		if err != nil {
			log.Println(err)
			txError(tx, err)
			return
		}
	}
	tx.Commit()

	return
}

func (p *PostSearch) set(offset int, posts []*Post) {
	p.pl.Lock()
	defer p.pl.Unlock()
	if p.posts == nil {
		p.posts = make(map[int][]*Post)
	}
	//fmt.Println("Setting Offset = ", offset)
	p.posts[offset] = posts
}
