package handlers

import (
	"fmt"
	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
	"html/template"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	errMustBeAdmin = "Must be logged in as an admin"
)

var Templates *template.Template

func renderTemplate(w http.ResponseWriter, tmpl string, p interface{}) {
	err := Templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.NotFound(w, r)
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
	defaultPostsPerPage = 24
	defaultImageSize    = 250
	pageCount           = 30
	maxTagsPerPage      = 50
	performBenchmarks   = false
)

var allowedFiletypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"video/webm",
	"video/mp4",
}

func allowedContentType(contentType string) bool {
	for _, cType := range allowedFiletypes {
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
		if set < 0 {
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
	setC(w, "daemon", r.FormValue("daemon"))
	setC(w, "limit", r.FormValue("limit"))
	setC(w, "ImageSize", r.FormValue("ImageSize"))

	th := r.FormValue("thumbhover")
	if th == "on" {
		setC(w, "thumbhover", "true")
	} else {
		setC(w, "thumbhover", "false")
	}

	thf := r.FormValue("thumbhoverfull")
	if thf == "on" {
		setC(w, "thumbhoverfull", "true")
	} else {
		setC(w, "thumbhoverfull", "false")
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func splitURI(uri string) []string {
	str := strings.Trim(uri, "/")
	return strings.Split(str, "/")
}

type Comment struct {
	ID   int
	User User
	Text string
	Time string
}

func tComment(c *DM.Comment) Comment {
	return Comment{c.ID, tUser(c.User), c.Text, c.Time}
}

func tComments(cm []*DM.Comment) (r []Comment) {
	for _, c := range cm {
		r = append(r, tComment(c))
	}
	return
}

func tPostComment(c *DM.PostComment) Comment {
	return Comment{c.ID, tUser(c.User), c.Text, c.Time}
}

func tPostComments(cm []*DM.PostComment) (r []Comment) {
	for _, c := range cm {
		r = append(r, tPostComment(c))
	}
	return
}

func CommentWallHandler(w http.ResponseWriter, r *http.Request) {
	user, uinfo := getUser(w, r)
	if r.Method == http.MethodPost {
		text := r.FormValue("text")
		if len(text) < 3 || len(text) > 7500 {
			http.Error(w, "Minimum 3 characters. Maximum 7500 characters.", http.StatusBadRequest)
			return
		}

		var c DM.Comment
		err := c.Save(user.QID(DM.DB), text)
		if err != nil {
			if err.Error() == "Post does not exist" {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			log.Println(err)
		}

		http.Redirect(w, r, "/wall", http.StatusSeeOther)
	}

	bm := benchmark.Begin()
	var commMod DM.CommentCollector
	err := commMod.Get(DM.DB, 100, uinfo.IpfsDaemon)
	if err != nil {
		http.Error(w, "Oops, something went wrong.", http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	type P struct {
		Username   string
		Comments   []Comment
		ServerTime string
		Time       string
	}
	var p P
	p.Comments = tComments(commMod.Comments)
	p.Username = user.QName(DM.DB)
	if p.Username == "" {
		p.Username = "Anonymous"
	}

	p.ServerTime = time.Now().Format(DM.Sqlite3Timestamp)
	p.Time = bm.EndStr(performBenchmarks)
	renderTemplate(w, "comments", p)
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
