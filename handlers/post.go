package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
	"github.com/kycklingar/mimemagic"
)

type Postpage struct {
	Base     base
	Post     *DM.Post
	Comments []*DM.PostComment
	Dups     *DM.Duplicate
	Comics   []*DM.Comic
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
			fmt.Println("Failed converting string to int")
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

	p.QHash(DM.DB)
	p.QDeleted(DM.DB)
	p.QID(DM.DB)
	p.QSize(DM.DB)
	p.QMime(DM.DB).QType(DM.DB)
	p.QMime(DM.DB).QName(DM.DB)
	p.QThumbnails(DM.DB)
	p.QDescription(DM.DB)

	pp.Post = p

	var tc DM.TagCollector
	err = tc.GetPostTags(DM.DB, p)
	if err != nil {
		log.Print(err)
	}

	pp.Sidebar.Tags = tc.Tags

	pp.Comments = p.Comments(DM.DB)

	for _, c := range pp.Comments {
		c.User.QFlag(DM.DB)
		c.User.QName(DM.DB)
	}

	pp.Comics = p.Comics(DM.DB)
	for _, c := range pp.Comics {
		c.QID(DM.DB)
		c.QTitle(DM.DB)
		c.QPageCount(DM.DB)
		if c.QChapterCount(DM.DB) <= 0 {
			continue
		}
		ch := c.QChapters(DM.DB)[0]
		for i, cp := range ch.QPosts(DM.DB) {
			if i >= 5 {
				break
			}
			cp.Post.QID(DM.DB)
			cp.Post.QThumbnails(DM.DB)
			cp.Post.QHash(DM.DB)
			cp.Post.QMime(DM.DB).QName(DM.DB)
			cp.Post.QMime(DM.DB).QType(DM.DB)
		}
	}

	pp.Base.Title = strconv.Itoa(pp.Post.ID)

	for _, tag := range pp.Sidebar.Tags {
		if tag.QNamespace(DM.DB).QNamespace(DM.DB) == "creator" {
			pp.Base.Title += " - " + tag.QTag(DM.DB)
		}
	}

	pp.Dups = p.Duplicate()
	pp.Dups.QID(DM.DB)
	for _, p := range pp.Dups.QPosts(DM.DB) {
		p.QID(DM.DB)
		p.QHash(DM.DB)
		p.QThumbnails(DM.DB)
		p.QDeleted(DM.DB)
	}

	pp.Time = bm.EndStr(performBenchmarks)
	renderTemplate(w, "post", pp)
}

func PostsHandler(w http.ResponseWriter, r *http.Request) {
	var p PostsPage

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

	type arg struct {
		name  string
		value string
	}

	args := []arg{}
	args = append(args, arg{"filter", p.Sidebar.Filter})
	args = append(args, arg{"unless", p.Sidebar.Unless})
	args = append(args, arg{"order", order})

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
	err = pc.Get(tagString, p.Sidebar.Filter, p.Sidebar.Unless, order)
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
		t.Namespace.QNamespace(DM.DB)
		p.SuggestedTags = append(p.SuggestedTags, t)
	}

	p.Sidebar.Tags = pc.Tags(maxTagsPerPage)
	for i := range p.Sidebar.Tags {
		p.Sidebar.Tags[i] = DM.CachedTag(p.Sidebar.Tags[i])
		p.Sidebar.Tags[i].QTag(DM.DB)
		p.Sidebar.Tags[i].QCount(DM.DB)
		p.Sidebar.Tags[i].QNamespace(DM.DB).SetCache()
		p.Sidebar.Tags[i].QNamespace(DM.DB).QNamespace(DM.DB)
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

	posts, err := post.FindSimilar(DM.DB, dist)
	if err != nil {
		//http.Error(w, ErrInternal, http.StatusInternalServerError)
		//return
	}

	var p PostsPage

	for _, pst := range posts {
		pst.QID(DM.DB)
		pst.QHash(DM.DB)
		pst.QThumbnails(DM.DB)
		pst.QDeleted(DM.DB)
		pst.QMime(DM.DB).QName(DM.DB)
		pst.QMime(DM.DB).QType(DM.DB)

		p.Posts = append(p.Posts, pst)
	}

	p.User = userCookies(w, r)
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "posts", p)
}
