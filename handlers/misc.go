package handlers

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"

	DM "github.com/kycklingar/PBooru/DataManager"
)

const (
	errMustBeAdmin = "Must be logged in as an admin"
)

func permErr(w http.ResponseWriter, perm string) {
	http.Error(w, lackingPermissions(perm), http.StatusBadRequest)
}

func lackingPermissions(priv string) string {
	return fmt.Sprintf("Insufficient privileges. Want %s", priv)
}

var Templates *template.Template

func renderTemplate(w http.ResponseWriter, tmpl string, p interface{}) {
	err := Templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		//log.Println(err)
	}
}

type base struct {
	Title string
}

type indexPage struct {
	Hits       int
	TotalPosts int
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI != "/" {
		notFoundHandler(w)
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	var p indexPage
	p.Hits = DM.Counter()
	p.TotalPosts = DM.GetTotalPosts()

	renderTemplate(w, "index", p)
}

// TODO: Put in config file
const (
	defaultPostsPerPage     = 24
	defaultImageSize        = 256
	defaultMinThumbnailSize = 0
	pageCount               = 30
	maxTagsPerPage          = 50
	performBenchmarks       = false
)

const (
	captchaNone = iota
	captchaAnon
	captchaEveryone
)

var (
	lts = 0
)

func largestThumbnailSize() int {
	if lts <= 0 {
		for _, s := range DM.CFG.ThumbnailSizes {
			if s > lts {
				lts = s
			}
		}
	}
	return lts
}

func allowedContentType(contentType string) bool {
	for _, cType := range CFG.AllowedMimes {
		if cType == contentType {
			return true
		}
	}
	return false
}

type Pageination struct {
	Pages   []int
	Current int
	Last    int
	First   int
	Prev    int
	Next    int
}

func pageinate(total, limit, currPage, numOfPages int) Pageination {
	var totalPages = int(math.Ceil(float64(total) / float64(limit)))
	var p Pageination
	p.Last = totalPages
	p.First = 1

	var halfPages = int(math.Ceil(float64(numOfPages) / 2.0))
	//fmt.Println(totalPages, total, limit)
	p.Current = currPage
	if currPage < halfPages { // Close to the beginning
		num := numOfPages
		if totalPages < num {
			num = totalPages
		}
		for i := 1; i <= num; i++ {
			p.Pages = append(p.Pages, i)
		}
	} else if totalPages-currPage < halfPages { // Close to the end
		set := totalPages - numOfPages + 1
		if set <= 0 {
			set = 1
		}
		for i := set; i <= totalPages; i++ {
			p.Pages = append(p.Pages, i)
		}
	} else { // In the middle
		set := currPage - halfPages + 1
		if set <= 0 {
			set = 1
		}
		for i := set; i <= currPage+halfPages; i++ {
			p.Pages = append(p.Pages, i)
		}
	}

	if currPage > 1 {
		p.Prev = currPage - 1
	}
	if currPage < totalPages {
		p.Next = currPage + 1
	}
	return p
}

func OptionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		user := userCookies(w, r)
		renderTemplate(w, "options", user)
		return
	}

	setCookie(w, "gateway", r.FormValue("gateway"), false)
	setCookie(w, "limit", r.FormValue("limit"), false)
	setCookie(w, "thumbnail_size", r.FormValue("thumbnail-size"), false)
	setCookie(w, "real_thumbnail_size", r.FormValue("real-thumbnail-size"), false)
	setCookie(w, "thumb_hover", r.FormValue("thumb-hover"), false)
	setCookie(w, "thumb_hover_full", r.FormValue("thumb-hover-full"), false)

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func splitURI(uri string) []string {
	str := strings.Trim(uri, "/")
	return strings.Split(str, "/")
}

type uri struct {
	raw        string
	components []string
}

func uriSplitter(r *http.Request) uri {
	return uri{
		raw: r.URL.EscapedPath(),
		components: strings.Split(
			strings.Trim(
				r.URL.EscapedPath(),
				"/",
			),
			"/",
		),
	}
}

type uriLength struct {
	index, length int
}

func (ue uriLength) Error() string {
	return fmt.Sprintf("index out of bounds. uri length %d, index %d", ue.length, ue.index)
}

func (u uri) length() int {
	return len(u.components)
}

func (u uri) getIntAtIndex(index int) (int, error) {
	if length := len(u.components); length <= index {
		return 0, uriLength{index, length}
	}
	return strconv.Atoi(u.components[index])
}

func (u uri) getAtIndex(index int) (string, error) {
	if length := len(u.components); length <= index {
		return "", uriLength{index, length}
	}

	return u.components[index], nil
}

type Comment struct {
	ID           int
	User         *DM.User
	Text         string
	CompiledText string
	Time         string
	Editable     bool
}

//func tComment(user *DM.User, c *DM.Comment) Comment {
//	c.User.QName(DM.DB)
//	return Comment{
//		c.ID,
//		c.User,
//		c.Text,
//		c.CompiledText,
//		c.Time.String(),
//		canEditComment(commentEditTimeoutMinutes, user, c),
//	}
//}
//
//func tComments(user *DM.User, cm []*DM.Comment) (r []Comment) {
//	for _, c := range cm {
//		r = append(r, tComment(user, c))
//	}
//	return
//}

func tPostComment(c *DM.PostComment) Comment {
	return Comment{c.ID, c.User, c.Text, c.Text, c.Time, false}
}

func tPostComments(cm []*DM.PostComment) (r []Comment) {
	for _, c := range cm {
		r = append(r, tPostComment(c))
	}
	return
}

func UrlEncode(uri string) string {
	res := strings.Replace(uri, "%", "%25", -1)
	res = strings.Replace(res, "/", "%25-2F", -1)
	res = strings.Replace(res, "?", "%25-3F", -1)
	res = strings.Replace(res, ".", "%25-D", -1)
	return res
}

func UrlDecode(uri string) string {
	res := strings.Replace(uri, "%-2F", "/", -1)
	res = strings.Replace(res, "%-3F", "?", -1)
	res = strings.Replace(res, "%-D", ".", -1)
	return res
}

func Max(x, y int) int {
	if x > y {
		return x
	}

	return y
}

func Min(x, y int) int {
	if x < y {
		return x
	}

	return y
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
