package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/user"
	"github.com/kycklingar/PBooru/DataManager/user/pool"
	"github.com/kycklingar/PBooru/benchmark"
	postform "github.com/kycklingar/PBooru/handlers/forms/post"
)

var errNoID = errors.New("No post identification provided")

func postFromForm(r *http.Request) (*DM.Post, error) {
	var formNames = []string{
		"post-id",
		"post-cid",
		"post-sha256",
		"post-md5",
	}

	var f, v string

	for _, name := range formNames {
		if v = r.FormValue(name); v != "" {
			f = name
			break
		}
	}

	var (
		p   *DM.Post
		err error
	)

	switch f {
	case "post-id":
		p = DM.NewPost()
		p.ID, err = strconv.Atoi(v)
	case "post-cid":
		p, err = DM.GetPostFromCID(v)
	case "post-sha256":
		p, err = DM.GetPostFromHash("sha256", v)
	case "post-md5":
		p, err = DM.GetPostFromHash("md5", v)
	default:
		err = errNoID
	}

	return p, err
}

func postError(w http.ResponseWriter, err error) {
	switch err {
	case sql.ErrNoRows:
		notFoundHandler(w)
	case errNoID:
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type Postpage struct {
	Base      base
	Post      *DM.Post
	Voted     bool
	Dns       []DM.DnsCreator
	Comments  []*DM.PostComment
	Dupe      DM.Dupe
	Alts      []*DM.Post
	Chapters  []*DM.Chapter
	Sidebar   Sidebar
	User      user.User
	UserPools pool.Pools
	UserInfo  UserInfo
	Time      string
}

var catMap = map[DM.Namespace]int{
	"creator":   0,
	"rating":    1,
	"gender":    2,
	"character": 3,
	"species":   4,
	"series":    5,
}

type postAndTags struct {
	Post *DM.Post

	// Namespaced tags displayed first
	Namespace [][]DM.Tag

	// The rest
	Tags []DM.Tag
}

type PostsPage struct {
	Base base
	//Result	      DM.SearchResult
	ErrorMessage  string
	Result        []postAndTags
	Sidebar       Sidebar
	SuggestedTags []DM.Tag
	//ArgString     string
	Pageinator Pageination
	User       UserInfo
	Time       string
}

func PostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user, _ := getUser(w, r)

		if user.ID == 0 {
			http.Error(w, "You must login to do that", http.StatusBadRequest)
			return
		}
		uri := splitURI(r.URL.Path)

		post := DM.NewPost()

		if len(uri) <= 1 {
			notFoundHandler(w)
			return
		} else if len(uri) >= 2 && uri[1] != "hash" {
			var err error
			postID, err := strconv.Atoi(uri[1])
			if err != nil {
				notFoundHandler(w)
				//log.Println("Failed converting string to int")
				return
			}
			err = post.SetID(DM.DB, postID)
			if err != nil {
				notFoundHandler(w)
				return
			}
		} else {
			notFoundHandler(w)
			return
		}

		post = DM.CachedPost(post)

		if r.FormValue("comment") == "true" {
			pc := post.NewComment()
			pc.User = user
			pc.Text = r.FormValue("text")

			internalError(w, pc.Save(DM.DB))

			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}
		//if !user.Flag.Tagging() {
		//	http.Error(w, "Insufficent privileges. Want 'Tagging'", http.StatusBadRequest)
		//	return
		//}

		//tagStrAdd := r.FormValue("addtags")
		//tagStrRem := r.FormValue("remtags")

		//tags := r.FormValue("tags")

		//err := post.EditTags(user, tags)
		//if internalError(w, err) {
		//	return
		//}
		// http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID(DM.DB), post.Hash(DM.DB)), http.StatusSeeOther)
		//http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
		http.Error(w, errors.New("Temporarily disabled").Error(), http.StatusInternalServerError)
		return
	}

	bm := benchmark.Begin()

	var pp Postpage
	pp.Sidebar.Mimes = make(map[string][]*DM.Mime)
	for _, mime := range DM.Mimes {
		pp.Sidebar.Mimes[mime.Type] = append(pp.Sidebar.Mimes[mime.Type], mime)
	}

	pp.User, pp.UserInfo = getUser(w, r)

	// Valid Uris: 	post/1
	//		post/hash/Qm...
	//		post/md5/HASH...
	//		post/sha256/HASH...
	var p = DM.NewPost()

	uri := uriSplitter(r)

	method, err := uri.getAtIndex(1)
	if err != nil {
		log.Println(err)
	}

	switch method {
	case "ipfs":
		var cid string
		cid, err = uri.getAtIndex(2)
		p, err = DM.GetPostFromCID(cid)
	case "sha256", "md5":
		var hash string
		hash, err = uri.getAtIndex(2)
		p, err = DM.GetPostFromHash(method, hash)
	default:
		var id int
		id, err = uri.getIntAtIndex(1)
		if err != nil {
			break
		}

		err = p.SetID(DM.DB, id)
	}

	if err != nil {
		postError(w, err)
		return
	}

	err = p.QMul(
		DM.DB,
		DM.PFCid,
		DM.PFChecksums,
		DM.PFRemoved,
		DM.PFDeleted,
		DM.PFSize,
		DM.PFTimestamp,
		DM.PFMime,
		DM.PFDimension,
		DM.PFThumbnails,
		DM.PFDescription,
		DM.PFMetaData,
	)
	if internalError(w, err) {
		return
	}

	pp.Post = p

	pp.Dupe, err = p.Duplicates(DM.DB)
	if internalError(w, err) {
		return
	}

	if err = pp.Dupe.Post.QMul(
		DM.DB,
		DM.PFCid,
		DM.PFRemoved,
		DM.PFThumbnails,
		DM.PFAlts,
		DM.PFAltGroup,
		DM.PFScore,
		DM.PFTombstone,
	); err != nil {
		log.Println(err)
	}

	for _, p := range pp.Dupe.Inferior {
		if err = p.QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFRemoved,
		); err != nil {
			log.Println(err)
		}
	}

	for i := 0; i < 5 && i < len(pp.Dupe.Post.Alts); i++ {
		pp.Alts = append(pp.Alts, pp.Dupe.Post.Alts[i])
	}

	pp.Voted, err = DM.HasVoted(pp.User.ID, pp.Dupe.Post.ID)

	pp.UserPools, err = pool.OfUser(r.Context(), pp.User.ID)
	if internalError(w, err) {
		return
	}

	bm.Split("Queriying tags")
	tags, err := pp.Dupe.Post.Tags()
	if internalError(w, err) {
		return
	}

	pp.Base.Title = strconv.Itoa(pp.Post.ID)

	for _, tag := range tags {
		if tag.Namespace == "creator" {
			pp.Base.Title += " - " + tag.Tag
			dns, err := DM.DnsGetCreatorFromTag(tag.ID)
			if err != nil {
				continue
			}

			pp.Dns = append(pp.Dns, dns)
		}
	}

	pp.Sidebar.Tags = tags

	//pp.Comments, err = p.Comments(DM.DB)
	if internalError(w, err) {
		return
	}

	pp.Chapters, err = DM.GetPostChapters(pp.Dupe.Post.ID)
	if internalError(w, err) {
		return
	}

	DM.RegisterPostView(pp.Dupe.Post.ID)

	pp.Time = bm.EndStr(performBenchmarks)
	renderTemplate(w, "post", pp)
}

func assignAltsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.Flag.Upload() {
		permErr(w, "Upload")
		return
	}

	ua := DM.UserAction(user)

	var pids []int

	r.ParseMultipartForm(1024 * 10)
	for _, v := range r.Form["post-id"] {
		id, err := strconv.Atoi(v)
		if badRequest(w, err) {
			return
		}

		pids = append(pids, id)
	}

	ua.Add(DM.SetAlts(pids))

	if internalError(w, ua.Exec()) {
		return
	}

	toRefOrBackup(w, r, fmt.Sprintf("/post/%d/", pids[0]))
}

func splitAltsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.Flag.Upload() {
		permErr(w, "Upload")
		return
	}

	ua := DM.UserAction(user)

	var pids []int

	r.ParseForm()
	for _, v := range r.Form["post-id"] {
		id, err := strconv.Atoi(v)
		if badRequest(w, err) {
			return
		}

		pids = append(pids, id)
	}

	ua.Add(DM.SplitAlts(pids))

	if internalError(w, ua.Exec()) {
		return
	}

	toRefOrBackup(w, r, fmt.Sprintf("/post/%d/", pids[0]))

}

//func assignAltsHandler(w http.ResponseWriter, r *http.Request) {
//	if r.Method != http.MethodPost {
//		notFoundHandler(w)
//		return
//	}
//
//	user, _ := getUser(w, r)
//	if !user.Flag.Upload() {
//		permErr(w, "Upload")
//		return
//	}
//
//	var p *DM.Post
//
//	r.ParseForm()
//	for _, v := range r.Form["post-id"] {
//		id, err := strconv.Atoi(v)
//		if err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//			return
//		}
//
//		if p != nil {
//			err = p.SetAlt(DM.DB, id, user)
//			if err != nil {
//				http.Error(w, err.Error(), http.StatusInternalServerError)
//				return
//			}
//		} else {
//			p = DM.NewPost()
//			p.ID = id
//		}
//	}
//
//	if p == nil {
//		http.Error(w, "No post-id's provided", http.StatusBadRequest)
//		return
//	}
//
//	http.Redirect(w, r, fmt.Sprintf("/post/%d/", p.ID), http.StatusSeeOther)
//}
//
//func unassignAltHandler(w http.ResponseWriter, r *http.Request) {
//	if r.Method != http.MethodPost {
//		notFoundHandler(w)
//		return
//	}
//
//	user, _ := getUser(w, r)
//	if !user.Flag.Upload() {
//		permErr(w, "Upload")
//		return
//	}
//
//	post, err := postFromForm(r)
//	if err != nil {
//		postError(w, err)
//		return
//	}
//
//	err = post.RemoveAlt(DM.DB, user)
//	if err != nil {
//		http.Error(w, err.Error(), http.StatusInternalServerError)
//		return
//	}
//
//	ref := r.FormValue("ref")
//	if ref == "" {
//		ref = fmt.Sprintf("/post/%d/", post.ID)
//	}
//
//	http.Redirect(w, r, ref, http.StatusSeeOther)
//}

func generateThumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)

	if !user.Flag.Delete() {
		permErr(w, "Delete")
		return
	}

	post, err := postFromForm(r)
	if err != nil {
		postError(w, err)
		return
	}

	err = DM.GenerateThumbnail(post.ID)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID, post.Cid), http.StatusSeeOther)
}

func postAddTagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)

	if !user.Flag.Tagging() {
		permErr(w, "Tagging")
		return
	}

	post, err := postFromForm(r)
	if err != nil {
		postError(w, err)
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.AlterPostTags(post.ID, r.FormValue("tags"), ""))
	if internalError(w, ua.Exec()) {
		return
	}

	//if internalError(w, post.AddTags(user, r.FormValue("tags"))) {
	//	return
	//}

	http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID, post.Cid), http.StatusSeeOther)
}

func postRemoveTagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	if true {
		http.Error(w, errors.New("Temporarily disabled").Error(), http.StatusInternalServerError)
		return
	}
	user, _ := getUser(w, r)

	if !user.Flag.Tagging() {
		permErr(w, "Tagging")
		return
	}

	post, err := postFromForm(r)
	if err != nil {
		postError(w, err)
		return
	}

	//if internalError(w, post.RemoveTags(user, r.FormValue("tags"))) {
	//	return
	//}

	http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID, post.Cid), http.StatusSeeOther)
}

type tagSort []DM.Tag

func (t tagSort) Len() int {
	return len(t)
}

func (t tagSort) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t tagSort) Less(i, j int) bool {
	return t[i].String() < t[j].String()
}

func PostVoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	u, _ := getUser(w, r)

	post, err := postFromForm(r)
	if err != nil {
		postError(w, err)
		return
	}

	post.QMul(DM.DB, DM.PFCid)

	if internalError(w, post.Vote(DM.DB, u)) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID, post.Cid), http.StatusSeeOther)
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
			notFoundHandler(w)
			return
		}
		offset = (page - 1) * pageLimit
	}

	tagString := r.FormValue("tags")
	p.Sidebar.Or = r.FormValue("or")
	p.Sidebar.Filter = r.FormValue("filter")
	p.Sidebar.Unless = r.FormValue("unless")
	order := r.FormValue("order")
	tombstone := r.FormValue("tombstone") == "on"

	//p.Sidebar.Alts = r.FormValue("alts") == "on"

	for _, alt := range r.Form["alts"] {
		if alt == "off" {
			p.Sidebar.Alts = false
			r.Form.Del("alts")
			break
		} else if alt == "on" {
			p.Sidebar.Alts = true
		}

	}

	p.Sidebar.AltGroup, _ = strconv.Atoi(r.FormValue("alt-group"))

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

	// Cleanup empty keys
	for k := range r.Form {
		if r.FormValue(k) == "" {
			r.Form.Del(k)
		}
	}

	p.Sidebar.Form = r.Form
	p.Sidebar.Form.Del("tags")

	if tagString != "" {
		red := fmt.Sprintf("/posts/%d/%s%s", page, PathEscape(tagString), "?"+r.Form.Encode())
		http.Redirect(w, r, red, http.StatusSeeOther)
		return
	}

	if len(uri) > 2 {
		tagString = PathUnescape(uri[2])
	}

	// var totalPosts int
	var err error

	bm.Split("Before posts")

	pc := DM.NewPostCollector()
	err = pc.Get(
		DM.SearchOptions{
			And:        tagString,
			Or:         p.Sidebar.Or,
			Filter:     p.Sidebar.Filter,
			Unless:     p.Sidebar.Unless,
			MimeIDs:    mimeIDs,
			Altgroup:   p.Sidebar.AltGroup,
			AltCollect: p.Sidebar.Alts,
			Tombstone:  tombstone,
			Order:      order,
		},
	)
	if err != nil {
		p.ErrorMessage = err.Error()
		//log.Println(err)
		// notFoundHandler(w, r)
		// return
	}

	pc = DM.CachedPostCollector(pc)

	//fmt.Println(pc.TotalPosts)
	result, err := pc.Search2(pageLimit, offset)
	if err != nil {
		log.Println(err)
		return
	}

	for _, set := range result {
		var pt = postAndTags{
			Post:      set.Post,
			Namespace: make([][]DM.Tag, len(catMap)),
		}

		if p.Sidebar.Alts {
			pt.Post.QMul(
				DM.DB,
				DM.PFCid,
				DM.PFMime,
				DM.PFScore,
				DM.PFTimestamp,
				DM.PFThumbnails,
				DM.PFAlts,
				DM.PFAltGroup,
			)
		} else {
			pt.Post.QMul(
				DM.DB,
				DM.PFCid,
				DM.PFMime,
				DM.PFScore,
				DM.PFChecksums,
				DM.PFTimestamp,
				DM.PFThumbnails,
			)
		}

		for _, tag := range set.Tags {
			if v, ok := catMap[tag.Namespace]; ok {
				pt.Namespace[v] = append(pt.Namespace[v], tag)
			} else {
				pt.Tags = append(pt.Tags, tag)
			}

		}

		p.Result = append(p.Result, pt)
	}
	//p.Result = result

	p.Sidebar.TotalPosts = pc.TotalPosts

	bm.Split("After posts")

	p.SuggestedTags, err = DM.TagHints(
		strings.Join(
			[]string{
				tagString,
				p.Sidebar.Or,
				p.Sidebar.Filter,
				p.Sidebar.Unless,
			},
			"\n",
		),
	)
	if internalError(w, err) {
		return
	}

	p.Sidebar.Tags = pc.Tags(maxTagsPerPage)

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
		renderTemplate(w, "upload", user)
	} else if r.Method == http.MethodPost {
		user, _ := getUser(w, r)
		if user.ID == 0 {
			http.Error(w, "You must login in order to upload", http.StatusForbidden)
			return
		}

		if !user.Flag.Upload() {
			http.Error(w, "Insufficient priviliges, Upload needed", http.StatusForbidden)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, CFG.MaxFileSize+1000000)

		file, fh, err := r.FormFile("file")
		if internalError(w, err) {
			return
		}
		defer file.Close()

		var ud DM.UploadData
		ud.FileSize = fh.Size

		mime, err := mimetype.DetectReader(file)
		if internalError(w, err) {
			return
		}

		ud.Mime = mime.String()

		if !allowedContentType(ud.Mime) {
			http.Error(w, "Filetype not allowed: "+ud.Mime, http.StatusBadRequest)
			return
		}

		ud.TagStr = r.FormValue("tags")

		if _, err = file.Seek(0, 0); err != nil {
			log.Println(err)
			return
		}

		ud.MetaData = r.FormValue("metadata")
		ud.Description = r.FormValue("description")

		if r.FormValue("store-filename") == "on" {
			ud.MetaData = fmt.Sprintf("filename:%s\n%s", fh.Filename, ud.MetaData)
		}

		post, err := DM.CreatePost(file, user, ud)
		if internalError(w, err) {
			return
		}

		// FIXME
		if len(r.FormValue("chapter-id")) > 0 {
			r.Form.Add("post-id", strconv.Itoa(post.ID))
			r.Header.Set("Referer", fmt.Sprintf("/post/%d", post.ID))
			addComicPageHandler(w, r)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/post/%d", post.ID), http.StatusSeeOther)

	} else {
		notFoundHandler(w)
		return
	}
}

func RemovePostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user, _ := getUser(w, r)

		if !user.Flag.Delete() {
			http.Error(w, "Insufficient privileges. Want \"Delete\"", http.StatusInternalServerError)
			return
		}

		post, err := postFromForm(r)
		if err != nil {
			postError(w, err)
			return
		}

		err = post.QMul(DM.DB, DM.PFRemoved)
		if internalError(w, err) {
			return
		}

		if post.Removed {
			if err = post.Reinstate(DM.DB); err != nil {
				log.Println(err)
			}
		} else {
			if err = post.Remove(DM.DB); err != nil {
				log.Println(err)
			}
		}
		http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", post.ID, post.Cid), http.StatusSeeOther)
		return
	}

	notFoundHandler(w)
}

//func postHistoryHandler(w http.ResponseWriter, r *http.Request) {
//	u, ui := getUser(w, r)
//	u = DM.CachedUser(u)
//
//	u.Flag
//
//	spl := splitURI(r.URL.Path)
//	if len(spl) < 3 {
//		notFoundHandler(w)
//		return
//	}
//
//	id, err := strconv.Atoi(spl[2])
//	if badRequest(w, err) {
//		return
//	}
//
//	const limit = 10
//	page, _ := strconv.Atoi("page")
//
//	post := DM.NewPost()
//	post.ID = id
//
//	post = DM.CachedPost(post)
//
//	totalEdits, err := post.QTagHistoryCount(DM.DB)
//	if internalError(w, err) {
//		return
//	}
//
//	ths, err := post.TagHistory(DM.DB, limit, page*limit)
//	if internalError(w, err) {
//		return
//	}
//
//	var thp TagHistoryPage
//	thp.Base.Title = fmt.Sprint("Tag History for ", post.ID)
//	thp.History = ths
//	thp.UserInfo = ui
//	thp.Pageinator = pageinate(totalEdits, limit, page, 10)
//	thp.User = u
//
//	preloadTagHistory(thp.History)
//
//	renderTemplate(w, "taghistory", thp)
//}

//func NewDuplicateHandler(w http.ResponseWriter, r *http.Request) {
//	if r.Method != http.MethodPost {
//		notFoundHandler(w, r)
//		return
//	}
//
//	u, _ := getUser(w, r)
//	if !u.Flag.Delete() {
//		http.Error(w, "Insufficient privileges. Want \"Delete\"", http.StatusBadRequest)
//		return
//	}
//
//	dupIDStr := r.FormValue("duppostid")
//	pIDStr := r.FormValue("postid")
//	levelStr := r.FormValue("level")
//
//	level, err := strconv.Atoi(levelStr)
//	if err != nil {
//		log.Print(err)
//		http.Error(w, "level not integer", http.StatusBadRequest)
//		return
//	}
//	pID, err := strconv.Atoi(pIDStr)
//	if err != nil {
//		log.Print(err)
//		http.Error(w, "postid not integer", http.StatusBadRequest)
//		return
//	}
//	dPostID, _ := strconv.Atoi(dupIDStr)
//	var dupID int
//	if dPostID != 0 {
//		p := DM.NewPost()
//		p.SetID(DM.DB, dPostID)
//
//		di := DM.NewDuplicate()
//		di.Post = p
//
//		dupID = di.QDupID(DM.DB)
//
//		if dupID == 0 {
//			http.Error(w, "That post have no dupes", http.StatusInternalServerError)
//			return
//		}
//	}
//
//	d := DM.NewDuplicate()
//	err = d.Post.SetID(DM.DB, pID)
//	if err != nil {
//		log.Print(err)
//		http.Error(w, "Oops", http.StatusInternalServerError)
//		return
//	}
//
//	err = d.SetDupID(DM.DB, dupID)
//	if err != nil {
//		log.Print(err)
//		http.Error(w, "Oops", http.StatusInternalServerError)
//		return
//	}
//	d.Level = level
//
//	err = d.Save()
//	if err != nil {
//		log.Print(err)
//		http.Error(w, "Oops", http.StatusInternalServerError)
//		return
//	}
//
//	http.Redirect(w, r, fmt.Sprintf("/post/%d/%s", pID, d.Post.QHash(DM.DB)), http.StatusSeeOther)
//}

func findSimilarHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	distStr := r.FormValue("distance")
	removed := r.FormValue("removed") == "on"

	bm := benchmark.Begin()

	id, err := strconv.Atoi(idStr)
	if internalError(w, err) {
		return
	}

	dist, err := strconv.Atoi(distStr)
	if err != nil {
		dist = 5
	}

	dist = mm.Min(10, mm.Max(1, dist))

	post := DM.NewPost()
	if internalError(w, post.SetID(DM.DB, id)) {
		return
	}

	var p = struct {
		Id       int
		Distance int

		Posts    []*DM.Post
		UserInfo UserInfo

		Time string
	}{
		Id:       id,
		Distance: dist,
	}

	p.Posts, err = post.FindSimilar(DM.DB, dist, removed)
	if err != nil {
		//http.Error(w, ErrInternal, http.StatusInternalServerError)
		//return
	}

	for i := range p.Posts {
		p.Posts[i] = DM.CachedPost(p.Posts[i])

		p.Posts[i].QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFRemoved,
			DM.PFMime,
			DM.PFAltGroup,
		)
	}

	p.UserInfo = userCookies(w, r)
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "similar", p)
}

func postModifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)

	ua := DM.UserAction(user)

	r.ParseForm()

	if badRequest(w, postform.ProcessFormData(ua, r.Form)) {
		return
	}

	if internalError(w, ua.Exec()) {
		return
	}

	toRefOrBackup(w, r, fmt.Sprint("/post/edit/", r.Form.Get("post-id")))
}

func PostEditHandler(w http.ResponseWriter, r *http.Request) {
	uri := uriSplitter(r)

	_, ui := getUser(w, r)

	id, err := uri.getIntAtIndex(2)
	if internalError(w, err) {
		return
	}

	var post = DM.NewPost()
	post.ID = id
	post.QMul(
		DM.DB,
		DM.PFCid,
		DM.PFDescription,
		DM.PFMetaData,
		DM.PFMime,
	)

	tags, err := post.Tags()
	if internalError(w, err) {
		return
	}

	var p = struct {
		Post     *DM.Post
		Tags     []DM.Tag
		UserInfo UserInfo
	}{
		Post:     post,
		Tags:     tags,
		UserInfo: ui,
	}

	renderTemplate(w, "post-edit", p)
}
