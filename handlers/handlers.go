package handlers

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dchest/captcha"
)

type Config struct {
	AllowedMimes  []string
	IPFSDaemonMap map[string]string
}

func (c *Config) Default() {
	if c.IPFSDaemonMap == nil {
		c.IPFSDaemonMap = make(map[string]string)
	}

	c.AllowedMimes = []string{
		"image/png",
		"image/jpeg",
		"image/gif",
		"video/webm",
		"video/mp4",
	}

	c.IPFSDaemonMap["default"] = "http://localhost:8080"
}

var CFG *Config

const (
	ErrInternal = "Internal Server Error"
	ErrNotFound = "Not Found"
)

var Handlers map[string]func(http.ResponseWriter, *http.Request)

//var Handlers map[string]http.Handler

type stats struct {
	elements *statElement
	prev     []*statElement

	l *sync.Mutex
}

type statElement struct {
	elements map[string]*statElement

	l *sync.Mutex

	name  string
	count int
}

func (e *statElement) init(name string) {
	e.elements = make(map[string]*statElement)
	e.l = &sync.Mutex{}
	e.name = name
}

func (e *statElement) inc(path []string) {
	if len(path) <= 0 {
		return
	}

	if e.elements == nil {
		e.init(path[0])
	}

	e.count++
	//fmt.Println(e.name, e.count)

	if len(path) <= 1 {
		return
	}
	e.l.Lock()
	defer e.l.Unlock()
	if e.elements[path[1]] == nil {
		e.elements[path[1]] = &statElement{}
	}
	e.elements[path[1]].inc(path[1:])
}

func (e *statElement) print(i int) string {
	str := fmt.Sprintf("%s [%d]\n<ol>", e.name, e.count)
	for _, el := range e.elements {
		str += fmt.Sprintf("<li value=\"%d\">%s</li>", el.count, el.print(i+1))
	}

	return fmt.Sprint(str, "</ol>")
}

func (e *statElement) Print() string {
	return e.print(1)
}

func (s *stats) init() {
	s.l = &sync.Mutex{}
	s.elements = &statElement{}

	s.tick()
}

func (s *stats) inc(path []string) {
	s.l.Lock()

	var p []string
	p = append(p, "")
	for _, i := range path {
		p = append(p, "/"+i)
	}

	s.elements.inc(p)

	s.l.Unlock()
}

func (s *stats) Total() int {
	var total int
	s.l.Lock()
	defer s.l.Unlock()
	for _, i := range s.prev {
		total += i.count
	}
	return total
}

func (s *stats) tick() {

	s.l.Lock()

	if len(s.prev) >= 24 {
		// for i := 1; i < len(s.prev); i++ {
		// 	s.prev[i-1] = s.prev[i]
		// }
		// s.prev[len(s.prev)-1] = s.elements

		for i := len(s.prev) - 1; i > 0; i-- {
			s.prev[i] = s.prev[i-1]
		}
		s.prev[0] = s.elements
	} else {
		var tmp []*statElement
		tmp = append(tmp, s.elements)
		s.prev = append(tmp, s.prev...)
	}

	s.elements = &statElement{}

	s.l.Unlock()

	time.AfterFunc(time.Hour, s.tick)
}

var stat stats

func makeStatHandler(fn func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		go func() {
			s := splitURI(r.URL.Path)
			stat.inc(s)
		}()

		fn(w, r)
	}
}

func statisticsHandler(w http.ResponseWriter, r *http.Request) {
	var p = struct {
		Total int
		Stats []*statElement
	}{
		stat.Total(),
		stat.prev[:max(3, len(stat.prev))],
	}

	renderTemplate(w, "statistics", p)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	renderTemplate(w, "404", nil)
}

func wrap2(x, y string, xv, yv interface{}) map[string]interface{} {
	return map[string]interface{}{
		x: xv,
		y: yv,
	}
}

func init() {
	stat.init()

	Templates = template.New("")
	Templates.Funcs(template.FuncMap{
		"noescape":  func(x string) template.HTML { return template.HTML(x) },
		"urlEncode": UrlEncode,
		"urlDecode": UrlDecode,
		"wrap2":     wrap2})

	tmpls, err := loadTemplates("./templates/")
	if err != nil {
		log.Fatal(err)
	}
	if _, err = Templates.ParseFiles(tmpls...); err != nil {
		log.Fatal(err)
	}

	Handlers = make(map[string]func(http.ResponseWriter, *http.Request))
	//Handlers = make(map[string]http.Handler)

	Handlers["/stats/"] = makeStatHandler(statisticsHandler)

	Handlers["/"] = makeStatHandler(IndexHandler)
	Handlers["/post/"] = makeStatHandler(PostHandler)
	Handlers["/posts"] = makeStatHandler(PostsHandler)
	Handlers["/posts/"] = makeStatHandler(PostsHandler)
	Handlers["/upload"] = makeStatHandler(UploadHandler)
	Handlers["/upload/"] = makeStatHandler(UploadHandler)
	Handlers["/options"] = makeStatHandler(OptionsHandler)
	Handlers["/options/"] = makeStatHandler(OptionsHandler)
	Handlers["/tags/"] = makeStatHandler(TagsHandler)
	Handlers["/taghistory/"] = makeStatHandler(TagHistoryHandler)
	Handlers["/user/"] = makeStatHandler(UserHandler)
	Handlers["/user/pool/"] = makeStatHandler(UserPoolHandler)
	Handlers["/user/pools/"] = makeStatHandler(UserPoolsHandler)
	Handlers["/user/pools/append/"] = makeStatHandler(UserPoolAppendHandler)
	Handlers["/user/pools/add/"] = makeStatHandler(UserPoolAddHandler)
	Handlers["/login/"] = makeStatHandler(LoginHandler)
	Handlers["/logout/"] = makeStatHandler(LogoutHandler)
	Handlers["/register/"] = makeStatHandler(RegisterHandler)
	Handlers["/deletepost"] = makeStatHandler(RemovePostHandler)
	Handlers["/wall/"] = makeStatHandler(CommentWallHandler)
	Handlers["/comics/"] = makeStatHandler(ComicsHandler)
	Handlers["/comic/"] = makeStatHandler(ComicHandler)
	Handlers["/comic/add"] = makeStatHandler(ComicAddHandler)
	Handlers["/comic/edit"] = makeStatHandler(EditComicHandler)
	Handlers["/links/"] = makeStatHandler(func(w http.ResponseWriter, r *http.Request) { renderTemplate(w, "links", nil) })

	Handlers["/dups/add/"] = makeStatHandler(NewDuplicateHandler)
	Handlers["/admin"] = makeStatHandler(func(w http.ResponseWriter, r *http.Request) {
		user, info := getUser(w, r)
		p := struct {
			UserInfo UserInfo
			User     User
		}{info, tUser(user)}

		renderTemplate(w, "admin", p)
	})
	Handlers["/slideshow"] = makeStatHandler(func(w http.ResponseWriter, r *http.Request) {
		_, ui := getUser(w, r)
		renderTemplate(w, "slideshow", ui)
	})

	Handlers["/similar/"] = makeStatHandler(findSimilarHandler)

	Handlers["/api/"] = makeStatHandler(APIHandler)
	Handlers["/api/v1/"] = makeStatHandler(APIv1Handler)
	Handlers["/api/v1/post"] = makeStatHandler(APIv1PostHandler)
	Handlers["/api/v1/posts"] = makeStatHandler(APIv1PostsHandler)
	Handlers["/api/v1/duplicate"] = makeStatHandler(APIv1DuplicateHandler)
	Handlers["/api/v1/suggesttags"] = makeStatHandler(APIv1SuggestTagsHandler)
	Handlers["/api/v1/similar"] = makeStatHandler(APIv1SimilarPostsHandler)

	Handlers["/captcha/"] = makeStatHandler(captcha.Server(150, 64).ServeHTTP)
	//Handlers["/verify/"] = verifyCaptcha
	//Handlers["/test/"] = testHandler

	//http.HandleFunc("/ssl"] = RedirectHandler
	// http.HandleFunc("/upload2/", uploadHandler2
	// http.HandleFunc("/ipfsdir/"] = IpfsDirFromCurrentSearchHandler
}

func loadTemplates(path string) ([]string, error) {
	paths, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range paths {
		if p.IsDir() {
			// fmt.Println(p.Name() + "/")
			n, err := loadTemplates(path + "/" + p.Name())
			if err != nil {
				return nil, err
			}
			names = append(names, n...)
		} else {
			// fmt.Println(p.Name())
			names = append(names, path+"/"+p.Name())
		}
	}
	return names, nil
}

func verifyCaptcha(w http.ResponseWriter, r *http.Request) bool {
	key := r.FormValue("key")
	code := r.FormValue("code")

	return captcha.VerifyString(key, code)
}

// func IpfsDirFromCurrentSearchHandler(w http.ResponseWriter, r *http.Request) {
// 	tagString := r.FormValue("tags")
// 	if tagString == "" {
// 		http.Error(w, "no tags", http.StatusInternalServerError)
// 		return
// 	}
// 	fmt.Println(tagString)

// 	multihash, err := DM.GetIpfsPageFromSearch(tagString)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	fmt.Fprint(w, multihash)
// }

// func createAnAdminisrator(w http.ResponseWriter, r *http.Request) {
// 	secretKey := r.FormValue("key")
// 	userid := r.FormValue("id")

// 	userID, err := strconv.Atoi(userid)

// 	if err != nil || userID == 0 || secretKey == "" || secretKey != "passwordforanewadmin" {
// 		fmt.Println(err, userID, secretKey)
// 		http.Error(w, "fail", http.StatusInternalServerError)
// 		return
// 	}

// 	err = DM.UpgradeUserToAdmin(userID)
// 	if err != nil {
// 		fmt.Println(err)
// 		http.Error(w, "error", http.StatusInternalServerError)
// 	}
// 	http.Redirect(w, r, "/posts/", http.StatusSeeOther)
// }
