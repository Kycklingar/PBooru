package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
	"github.com/kycklingar/mimemagic"
)

type Postpage struct {
	Base     base
	Post     *DM.Post
	Voted    bool
	Comments []*DM.PostComment
	Dupe     DM.Dupe
	//Comics   []*DM.Comic
	Chapters []*DM.Chapter
	Sidebar  Sidebar
	User     *DM.User
	UserInfo UserInfo
	Time     string
}

type PostsPage struct {
	Base          base
	Posts         []*DM.Post
	Sidebar       Sidebar
	SuggestedTags []*DM.Tag
	ArgString     string
	Pageinator    Pageination
	User          UserInfo
	Time          string
}

func PostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user, _ := getUser(w, r)

		if user.QID(DM.DB) == 0 {
			http.Error(w, "You must login to do that", http.StatusBadRequest)
			return
		}
		uri := splitURI(r.URL.Path)

		post := DM.NewPost()

		if len(uri) <= 1 {
			notFoundHandler(w, r)
			return
		} else if len(uri) >= 2 && uri[1] != "hash" {
			var err error
			postID, err := strconv.Atoi(uri[1])
			if err != nil {
				notFoundHandler(w, r)
				//log.Println("Failed converting string to int")
				return
			}
			err = post.SetID(DM.DB, postID)
			if err != nil {
				notFoundHandler(w, r)
				return
			}
		} else {
			notFoundHandler(w, r)
			return
		}

		if r.FormValue("comment") == "true" {
			pc := post.NewComment()
			pc.User = user
			pc.Text = r.FormValue("text")

			if err := pc.Save(DM.DB); err != nil {
				log.Println(err)
				http.Error(w, "Oops", http.StatusInternalServerError)
			}
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
		if !user.QFlag(DM.DB).Tagging() {
			http.Error(w, "Insufficent privileges. Want 'Tagging'", http.StatusBadRequest)
			return
		}

		tagStrAdd := r.FormValue("addtags")
		tagStrRem := r.FormValue("remtags")

		err := post.EditTags(user, tagStrAdd, tagStrRem)
		if err != nil {
			log.Print(err)
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		// http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID(DM.DB), post.Hash(DM.DB)), http.StatusSeeOther)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	bm := benchmark.Begin()

	var pp Postpage
	pp.Sidebar.Mimes = make(map[string][]*DM.Mime)
	for _, mime := range DM.Mimes {
		pp.Sidebar.Mimes[mime.Type] = append(pp.Sidebar.Mimes[mime.Type], mime)
	}
	p := DM.NewPost()

	pp.User, pp.UserInfo = getUser(w, r)

	pp.User.QID(DM.DB)
	pp.User.QFlag(DM.DB)
	pp.User.QPools(DM.DB)

	uri := splitURI(r.URL.Path)

	// Valid Uris: 	post/1
	//		post/hash/Qm...
	if len(uri) <= 1 {
		notFoundHandler(w, r)
		return
	} else if len(uri) >= 2 && uri[1] != "hash" {
		id, err := strconv.Atoi(uri[1])
		if err != nil {
			notFoundHandler(w, r)
			//fmt.Println("Failed converting string to int")
			return
		}

		err = p.SetID(DM.DB, id)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		//dPost = DM.NewPost(id)
		if p.QID(DM.DB) == 0 {
			notFoundHandler(w, r)
			return
		}

		bm.Split("Posts added")

	} else if len(uri) > 2 && uri[1] == "hash" {
		p.Hash = uri[2]
		if p.QID(DM.DB) == 0 {
			notFoundHandler(w, r)
			return
		}
	}
	var err error

	p.QID(DM.DB)

	p = DM.CachedPost(p)
	p.QHash(DM.DB)
	p.QDeleted(DM.DB)
	p.QSize(DM.DB)
	p.QMime(DM.DB).QType(DM.DB)
	p.QMime(DM.DB).QName(DM.DB)
	p.QThumbnails(DM.DB)
	p.QDescription(DM.DB)
	p.QScore(DM.DB)

	pp.Post = p

	pp.Dupe, err = p.Duplicates(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pp.Dupe.Post = DM.CachedPost(pp.Dupe.Post)

	pp.Dupe.Post.QHash(DM.DB)
	pp.Dupe.Post.QDeleted(DM.DB)
	pp.Dupe.Post.QThumbnails(DM.DB)

	for _, p := range pp.Dupe.Inferior {
		p.QID(DM.DB)
		p = DM.CachedPost(p)
		p.QHash(DM.DB)
		p.QThumbnails(DM.DB)
		p.QDeleted(DM.DB)
	}

	pp.Voted = pp.User.Voted(DM.DB, p)

	var tc DM.TagCollector

	err = tc.GetFromPost(DM.DB, pp.Dupe.Post)
	if err != nil {
		log.Print(err)
	}

	for _, tag := range tc.Tags {
		tag.QTag(DM.DB)
		tag.QCount(DM.DB)
		tag.QNamespace(DM.DB).QNamespace(DM.DB)
	}

	pp.Sidebar.Tags = tc.Tags

	pp.Comments = p.Comments(DM.DB)

	for _, c := range pp.Comments {
		c.User.QFlag(DM.DB)
		c.User.QName(DM.DB)
	}

	pp.Chapters = pp.Dupe.Post.Chapters(DM.DB)
	for _, c := range pp.Chapters {
		c.QComic(DM.DB)
		c.Comic.QTitle(DM.DB)
		c.Comic.QPageCount(DM.DB)
		c.QTitle(DM.DB)
		c.QOrder(DM.DB)
		for i, p := range c.QPosts(DM.DB) {
			if i > 5 {
				break
			}
			p.QOrder(DM.DB)
			p.QPost(DM.DB)
			p.Post.QHash(DM.DB)
			p.Post.QThumbnails(DM.DB)
		}
	}

	pp.Base.Title = strconv.Itoa(pp.Post.ID)

	for _, tag := range pp.Sidebar.Tags {
		if tag.QNamespace(DM.DB).QNamespace(DM.DB) == "creator" {
			pp.Base.Title += " - " + tag.QTag(DM.DB)
		}
	}

	sortTags(pp.Sidebar.Tags)

	pp.Time = bm.EndStr(performBenchmarks)
	renderTemplate(w, "post", pp)
}

func sortTags(tags []*DM.Tag) {
	for i := 0; i < len(tags); i++ {
		for j := len(tags) - 1; j > i; j-- {
			tag1 := tags[i].Namespace.Namespace + ":" + tags[i].Tag
			tag2 := tags[j].Namespace.Namespace + ":" + tags[j].Tag
			if tags[i].Namespace.Namespace == "none" {
				tag1 = tags[i].Tag
			}
			if tags[j].Namespace.Namespace == "none" {
				tag2 = tags[j].Tag
			}

			if strings.Compare(tag1, tag2) == 1 {
				tmp := tags[i]
				tags[i] = tags[j]
				tags[j] = tmp
			}
		}
	}
}

func PostVoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	u, _ := getUser(w, r)

	postID, err := strconv.Atoi(r.FormValue("post-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var post = DM.NewPost()
	post.ID = postID
	post = DM.CachedPost(post)

	if err = post.Vote(DM.DB, u); err != nil {
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func PostsHandler(w http.ResponseWriter, r *http.Request) {
	var p PostsPage
	p.Sidebar.Mimes = make(map[string][]*DM.Mime)

	bm := benchmark.Begin()

	p.User = userCookies(w, r)

	pageLimit := p.User.Limit

	var offset = 0
	uri := splitURI(r.URL.Path)
	var page int
	if len(uri) <= 1 {
		page = 1

	} else {
		var err error
		page, err = strconv.Atoi(uri[1])
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		offset = (page - 1) * pageLimit
	}

	tagString := r.FormValue("tags")
	p.Sidebar.Filter = r.FormValue("filter")
	p.Sidebar.Unless = r.FormValue("unless")
	order := r.FormValue("order")

	for _, mime := range DM.Mimes {
		p.Sidebar.Mimes[mime.Type] = append(p.Sidebar.Mimes[mime.Type], mime)
	}

	mimeGroups := r.Form["mime-type"]

	mimeIDs := DM.MimeIDsFromType(mimeGroups)

	mimes := r.Form["mime"]
	for _, mime := range mimes {
		id, err := strconv.Atoi(mime)
		if err != nil {
			log.Println(err)
			continue
		}
		contains := func(s []int, i int) bool {
			for _, x := range s {
				if x == i {
					return true
				}
			}

			return false
		}

		if !contains(mimeIDs, id) {
			mimeIDs = append(mimeIDs, id)
		}
	}

	type arg struct {
		name  string
		value string
	}

	args := []arg{}
	args = append(args, arg{"filter", p.Sidebar.Filter})
	args = append(args, arg{"unless", p.Sidebar.Unless})
	args = append(args, arg{"order", order})

	for _, group := range mimeGroups {
		args = append(args, arg{"mime-type", group})
	}

	for _, mimeID := range mimes {
		args = append(args, arg{"mime", mimeID})
	}

	argString := func(arguments []arg) string {
		var str string
		for _, arg := range arguments {
			if arg.value != "" {
				str += fmt.Sprintf("%s=%s&", arg.name, arg.value)
			}
		}
		if str != "" {
			str = "?" + str
		}
		return str
	}

	p.ArgString = argString(args)

	if tagString != "" {
		red := fmt.Sprintf("/posts/%d/%s%s", page, UrlEncode(tagString), p.ArgString)
		http.Redirect(w, r, red, http.StatusSeeOther)
		return
	}

	if len(uri) > 2 {
		tagString = UrlDecode(uri[2])
	}

	// var totalPosts int
	var err error

	bm.Split("Before posts")

	pc := &DM.PostCollector{}
	err = pc.Get(tagString, p.Sidebar.Filter, p.Sidebar.Unless, order, mimeIDs)
	if err != nil {
		//log.Println(err)
		// notFoundHandler(w, r)
		// return
	}

	pc = DM.CachedPostCollector(pc)

	//fmt.Println(pc.TotalPosts)
	for _, post := range pc.Search(pageLimit, offset) {
		post.QMime(DM.DB).QName(DM.DB)
		post.QMime(DM.DB).QType(DM.DB)
		p.Posts = append(p.Posts, post)
	}
	p.Sidebar.TotalPosts = pc.TotalPosts

	bm.Split("After posts")

	var tc DM.TagCollector
	tc.Parse(tagString)

	for _, t := range tc.SuggestedTags(DM.DB).Tags {
		t.QTag(DM.DB)
		t.QNamespace(DM.DB).QNamespace(DM.DB)
		p.SuggestedTags = append(p.SuggestedTags, t)
	}

	//p.Sidebar.Tags = pc.Tags(maxTagsPerPage)
	sidebarTags := pc.Tags(maxTagsPerPage)
	p.Sidebar.Tags = make([]*DM.Tag, len(sidebarTags))
	for i, tag := range sidebarTags {
		//p.Sidebar.Tags[i] = DM.CachedTag(p.Sidebar.Tags[i])
		tag.QTag(DM.DB)
		tag.QCount(DM.DB)
		tag.QNamespace(DM.DB).QNamespace(DM.DB)

		//fmt.Println(tag.Tag, tag.Namespace)

		p.Sidebar.Tags[i] = tag
	}

	bm.Split("Retrieved and appended tags")

	p.Time = bm.EndStr(performBenchmarks)

	p.Base.Title = strconv.Itoa(page) + " / " + tagString + " -- " + strconv.Itoa(p.Sidebar.TotalPosts)
	p.Sidebar.Query = tagString

	p.Pageinator = pageinate(pc.TotalPosts, pageLimit, page, pageCount)
	//fmt.Println(float64(dur.Seconds() * 2000))
	//p.Time = fmt.Sprint(float64(int64(float64(dur.Seconds()*2000+0.5))) / 2000)

	renderTemplate(w, "posts", p)
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		user, _ := getUser(w, r)
		user.QFlag(DM.DB)
		renderTemplate(w, "upload", user)
	} else if r.Method == http.MethodPost {
		user, _ := getUser(w, r)
		if user.QID(DM.DB) == 0 {
			http.Error(w, "You must login in order to upload", http.StatusForbidden)
			return
		}
		user.QFlag(DM.DB)

		if !user.Flag().Upload() {
			http.Error(w, "Insufficient priviliges, Upload needed", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 51<<20)
		r.ParseMultipartForm(50 << 20)
		file, fh, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed retrieving file.", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil {
			return
		}

		file.Seek(0, 0)

		mime := mimemagic.MatchMagic(buffer)
		contentType := mime.MediaType()
		if !allowedContentType(contentType) {
			http.Error(w, "Filetype not allowed: "+contentType, http.StatusBadRequest)
			return
		}

		tagString := r.FormValue("tags")

		post := DM.NewPost()

		err = post.New(file, fh.Size, tagString, contentType, user)
		if err != nil {
			http.Error(w, "Oops, Something went wrong.", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/post/%d", post.QID(DM.DB)), http.StatusSeeOther)

	} else {
		notFoundHandler(w, r)
		return
	}
}

func RemovePostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user, _ := getUser(w, r)

		if !user.QFlag(DM.DB).Delete() {
			http.Error(w, "Insufficient privileges. Want \"Delete\"", http.StatusInternalServerError)
			return
		}

		postid := r.FormValue("postid")

		postID, err := strconv.Atoi(postid)
		if err != nil || postID <= 0 {
			http.Error(w, "postid is empty", http.StatusInternalServerError)
			return
		}

		var post = DM.NewPost()
		post.SetID(DM.DB, postID)
		if post.QDeleted(DM.DB) >= 1 {
			if err = post.UnDelete(DM.DB); err != nil {
				log.Println(err)
			}
		} else {
			if err = post.Delete(DM.DB); err != nil {
				log.Println(err)
			}
		}
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func postHistoryHandler(w http.ResponseWriter, r *http.Request) {
	u, ui := getUser(w, r)
	u = DM.CachedUser(u)

	u.QFlag(DM.DB)

	spl := splitURI(r.URL.Path)
	if len(spl) < 3 {
		notFoundHandler(w, r)
		return
	}

	id, err := strconv.Atoi(spl[2])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	const limit = 10
	page, _ := strconv.Atoi("page")

	post := DM.NewPost()
	post.ID = id

	post = DM.CachedPost(post)

	totalEdits, err := post.QTagHistoryCount(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	ths, err := post.TagHistory(DM.DB, limit, page*limit)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var thp TagHistoryPage
	thp.Base.Title = fmt.Sprint("Tag History for ", post.ID)
	thp.History = ths
	thp.UserInfo = ui
	thp.Pageinator = pageinate(totalEdits, limit, page, 10)
	thp.User = u

	preloadTagHistory(thp.History)

	renderTemplate(w, "taghistory", thp)
}

func NewDuplicateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	u, _ := getUser(w, r)
	if !u.QFlag(DM.DB).Delete() {
		http.Error(w, "Insufficient privileges. Want \"Delete\"", http.StatusBadRequest)
		return
	}

	dupIDStr := r.FormValue("duppostid")
	pIDStr := r.FormValue("postid")
	levelStr := r.FormValue("level")

	level, err := strconv.Atoi(levelStr)
	if err != nil {
		log.Print(err)
		http.Error(w, "level not integer", http.StatusBadRequest)
		return
	}
	pID, err := strconv.Atoi(pIDStr)
	if err != nil {
		log.Print(err)
		http.Error(w, "postid not integer", http.StatusBadRequest)
		return
	}
	dPostID, _ := strconv.Atoi(dupIDStr)
	var dupID int
	if dPostID != 0 {
		p := DM.NewPost()
		p.SetID(DM.DB, dPostID)

		di := DM.NewDuplicate()
		di.Post = p

		dupID = di.QDupID(DM.DB)

		if dupID == 0 {
			http.Error(w, "That post have no dupes", http.StatusInternalServerError)
			return
		}
	}

	d := DM.NewDuplicate()
	err = d.Post.SetID(DM.DB, pID)
	if err != nil {
		log.Print(err)
		http.Error(w, "Oops", http.StatusInternalServerError)
		return
	}

	err = d.SetDupID(DM.DB, dupID)
	if err != nil {
		log.Print(err)
		http.Error(w, "Oops", http.StatusInternalServerError)
		return
	}
	d.Level = level

	err = d.Save()
	if err != nil {
		log.Print(err)
		http.Error(w, "Oops", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", pID, d.Post.QHash(DM.DB)), http.StatusSeeOther)
}

func findSimilarHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	distStr := r.FormValue("distance")

	bm := benchmark.Begin()

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	dist, err := strconv.Atoi(distStr)
	if err != nil {
		dist = 5
	}

	dist = DM.Smal(10, DM.Larg(1, dist))

	post := DM.NewPost()
	if err = post.SetID(DM.DB, id); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	var p struct {
		Posts    []*DM.Post
		UserInfo UserInfo

		Time string
	}

	p.Posts, err = post.FindSimilar(DM.DB, dist)
	if err != nil {
		//http.Error(w, ErrInternal, http.StatusInternalServerError)
		//return
	}

	for i, _ := range p.Posts {
		p.Posts[i].QID(DM.DB)
		p.Posts[i] = DM.CachedPost(p.Posts[i])

		p.Posts[i].QHash(DM.DB)
		p.Posts[i].QThumbnails(DM.DB)
		p.Posts[i].QDeleted(DM.DB)
		p.Posts[i].QMime(DM.DB).QName(DM.DB)
		p.Posts[i].QMime(DM.DB).QType(DM.DB)
	}

	p.UserInfo = userCookies(w, r)
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "similar", p)
}
