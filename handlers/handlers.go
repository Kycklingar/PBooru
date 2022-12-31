package handlers

import (
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dchest/captcha"
	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/user"
)

type Config struct {
	AllowedMimes         []string
	MaxFileSize          int64
	IPFSDaemonMap        map[string]string
	EnableCommentCaptcha int
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
	c.EnableCommentCaptcha = captchaEveryone
	c.MaxFileSize = 50000000 // 50MB
}

var CFG *Config

const (
	ErrInternal = "Internal Server Error"
	ErrNotFound = "Not Found"
)

var (
	ErrPriv = func(name string) string {
		return fmt.Sprint("Insufficent privileges. Want: ", name)
	}
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
	str := fmt.Sprintf("%s [%d]\n<ul>", html.EscapeString(e.name), e.count)
	for _, el := range e.elements {
		str += fmt.Sprintf("<li>%s</li>", el.print(i+1))
	}

	return fmt.Sprint(str, "</ul>")
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
		stat.prev[:mm.Min(3, len(stat.prev))],
	}

	renderTemplate(w, "statistics", p)
}

func notFoundHandler(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	renderTemplate(w, "404", nil)
}

func wrap2(x, y string, xv, yv interface{}) map[string]interface{} {
	return map[string]interface{}{
		x: xv,
		y: yv,
	}
}

func add(x, y int) int {
	return x + y
}

func mul(x, y int) int {
	return x * y
}

func stripExtension(filename string) string {
	return strings.TrimSuffix(filename, path.Ext(filename))
}

func init() {
	stat.init()

	Templates = template.New("")
	Templates.Funcs(
		template.FuncMap{
			"noescape":  func(x string) template.HTML { return template.HTML(x) },
			"urlEncode": PathEscape,
			"urlDecode": PathUnescape,
			"wrap2":     wrap2,
			"add":       add,
			"mul":       mul,
			"colorID":   colorID,
			"random":    func(chance int) bool { return rand.Int()%chance == 0 },
			"stripExt":  stripExtension,
		},
	)

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

	Handlers["/spine/"] = makeStatHandler(searchLogsHandler)

	Handlers["/"] = makeStatHandler(IndexHandler)
	Handlers["/post/"] = makeStatHandler(PostHandler)
	Handlers["/post/edit/"] = makeStatHandler(PostEditHandler)
	Handlers["/post/edit/remove/"] = makeStatHandler(RemovePostHandler)
	Handlers["/post/edit/thumbnails/generate/"] = makeStatHandler(generateThumbnailsHandler)
	Handlers["/post/edit/tags/add/"] = makeStatHandler(postAddTagsHandler)
	Handlers["/post/edit/tags/remove/"] = makeStatHandler(postRemoveTagsHandler)
	//Handlers["/post/taghistory/"] = postHistoryHandler
	Handlers["/post/report/"] = reportPostHandler
	Handlers["/post/vote/"] = PostVoteHandler

	Handlers["/post/edit/alts/assign/"] = assignAltsHandler
	Handlers["/post/edit/alts/split/"] = splitAltsHandler
	Handlers["/post/edit/description/"] = postModifyHandler

	Handlers["/edit/many/post/tags/"] = multiTagsHandler

	Handlers["/posts"] = makeStatHandler(PostsHandler)
	Handlers["/posts/"] = makeStatHandler(PostsHandler)

	Handlers["/compare/"] = comparisonHandler
	Handlers["/compare2/"] = compare2Handler

	Handlers["/appletree/"] = appleTreeHandler

	Handlers["/reports/"] = makeStatHandler(reportsHandler)
	Handlers["/reports/delete/"] = reportDeleteHandler

	Handlers["/reports/duplicates/"] = dupReportsHandler
	Handlers["/reports/duplicates/cleanup"] = dupReportCleanupHandler
	Handlers["/duplicate/report/"] = dupReportHandler
	Handlers["/duplicate/process/"] = processReportHandler
	Handlers["/duplicate/compare/"] = compareReportHandler
	Handlers["/duplicate/pluck/"] = processPluckReportHandler

	Handlers["/upload"] = makeStatHandler(UploadHandler)
	Handlers["/upload/"] = makeStatHandler(UploadHandler)
	Handlers["/options"] = makeStatHandler(OptionsHandler)
	Handlers["/options/"] = makeStatHandler(OptionsHandler)
	Handlers["/tags/"] = makeStatHandler(TagsHandler)
	//Handlers["/taghistory/"] = makeStatHandler(TagHistoryHandler)
	//Handlers["/taghistory/reverse/"] = makeStatHandler(ReverseTagHistoryHandler)

	Handlers["/user/"] = makeStatHandler(UserHandler)
	Handlers["/user/edit/flag/"] = upgradeUserHandler
	//Handlers["/user/taghistory/"] = makeStatHandler(UserTagHistoryHandler)
	Handlers["/user/pool/"] = makeStatHandler(UserPoolHandler)
	Handlers["/user/pool/remove/"] = makeStatHandler(editUserPoolHandler)
	Handlers["/user/pools/"] = makeStatHandler(UserPoolsHandler)
	Handlers["/user/pools/append/"] = makeStatHandler(UserPoolAppendHandler)
	Handlers["/user/pools/add/"] = makeStatHandler(UserPoolAddHandler)

	Handlers["/user/message/"] = makeStatHandler(messageHandler)
	Handlers["/user/message/new/"] = makeStatHandler(sendMessageHandler)
	Handlers["/user/messages/"] = makeStatHandler(allMessagesHandler)
	Handlers["/user/messages/sent/"] = makeStatHandler(sentMessagesHandler)
	Handlers["/user/messages/new/"] = makeStatHandler(newMessagesHandler)
	//Handlers["/user/messages/read/"] = makeStatHandler(readMessagesHandler)

	Handlers["/login/"] = makeStatHandler(LoginHandler)
	Handlers["/logout/"] = makeStatHandler(LogoutHandler)
	Handlers["/register/"] = makeStatHandler(RegisterHandler)

	Handlers["/deletepost"] = makeStatHandler(RemovePostHandler)

	Handlers["/wall/"] = makeStatHandler(CommentWallHandler)
	Handlers["/wall/edit/"] = makeStatHandler(editCommentHandler)
	Handlers["/wall/delete/"] = makeStatHandler(deleteCommentHandler)

	Handlers["/comics/"] = makeStatHandler(ComicsHandler)
	Handlers["/comic/"] = makeStatHandler(comicHandler)
	Handlers["/comic/create/"] = makeStatHandler(createComicHandler)
	Handlers["/comic/delete/"] = makeStatHandler(deleteComicHandler)
	Handlers["/comic/edit/"] = makeStatHandler(editComicHandler)
	Handlers["/comic/chapter/add/"] = makeStatHandler(addChapterHandler)
	Handlers["/comic/chapter/edit/"] = makeStatHandler(editChapterHandler)
	Handlers["/comic/chapter/edit/shift/"] = makeStatHandler(chapterShiftHandler)
	Handlers["/comic/chapter/delete/"] = makeStatHandler(deleteChapterHandler)
	Handlers["/comic/chapter/page/add/"] = makeStatHandler(addComicPageHandler)
	Handlers["/comic/chapter/page/edit/"] = makeStatHandler(editComicPageHandler)
	Handlers["/comic/chapter/page/delete/"] = makeStatHandler(deleteComicPageHandler)
	//Handlers["/comic/chapter/edit/shift/"] = makeStatHandler(chapterShiftHandler)
	//Handlers["/comic/chapter/delete/"] = makeStatHandler(comicRemoveChapterHandler)
	//Handlers["/comic/page/add/"] = makeStatHandler(comicAddPageHandler)
	//Handlers["/comic/page/add2/"] = makeStatHandler(comicAddPageApiHandler)
	//Handlers["/comic/page/edit/"] = makeStatHandler(comicEditPageHandler)
	//Handlers["/comic/page/delete/"] = makeStatHandler(comicDeletePageHandler)
	Handlers["/links/"] = makeStatHandler(func(w http.ResponseWriter, r *http.Request) { renderTemplate(w, "links", nil) })
	Handlers["/lookup/"] = makeStatHandler(imageLookupHandler)

	Handlers["/tombstone/"] = makeStatHandler(tombstoneHandler)
	Handlers["/tombstone/search/"] = makeStatHandler(tombstoneSearchHandler)
	Handlers["/archive/create"] = createArchiveHandler
	Handlers["/archive/"] = archiveHandler

	//Handlers["/dups/add/"] = makeStatHandler(NewDuplicateHandler)
	Handlers["/admin"] = makeStatHandler(func(w http.ResponseWriter, r *http.Request) {
		u, info := getUser(w, r)
		p := struct {
			UserInfo UserInfo
			User     user.User
			Mimes    map[string][]*DM.Mime
		}{info, u, make(map[string][]*DM.Mime)}

		for _, mime := range DM.Mimes {
			p.Mimes[mime.Type] = append(p.Mimes[mime.Type], mime)
		}

		renderTemplate(w, "admin", p)
	})
	Handlers["/slideshow"] = makeStatHandler(func(w http.ResponseWriter, r *http.Request) {
		_, ui := getUser(w, r)
		renderTemplate(w, "slideshow", ui)
	})

	Handlers["/dns"] = dnsHandler
	Handlers["/dns/"] = dnsCreatorHandler

	Handlers["/dns/edit"] = dnsEditHandler
	Handlers["/dns/domain/new"] = specialMM(dnsNewDomain)
	Handlers["/dns/domain/edit"] = specialMM(dnsEditDomain)

	Handlers["/dns/banner/new"] = specialMM(dnsNewBanner)
	Handlers["/dns/url/new"] = specialMM(dnsAddUrl)
	Handlers["/dns/url/remove"] = specialMM(dnsRemoveUrl)

	Handlers["/dns/creator/new"] = specialMM(dnsNewCreator)
	Handlers["/dns/creator/name/edit"] = specialMM(dnsEditCreatorName)
	Handlers["/dns/creator/tag/edit"] = specialMM(dnsEditCreatorTags)
	Handlers["/dns/creator/tagmap/add"] = specialMM(dnsMapTag)

	Handlers["/dns/tag/new"] = specialMM(dnsTagCreate)
	Handlers["/dns/tag/edit"] = specialMM(dnsTagEdit)
	//Handlers["/dns/tag/remove"] = dnsTagRemove

	Handlers["/similar/"] = makeStatHandler(findSimilarHandler)

	Handlers["/api/"] = makeStatHandler(APIHandler)
	Handlers["/api/v1/"] = makeStatHandler(APIv1Handler)
	Handlers["/api/v1/post"] = makeStatHandler(APIv1PostHandler)
	Handlers["/api/v1/posts"] = makeStatHandler(APIv1PostsHandler)
	//Handlers["/api/v1/duplicate"] = makeStatHandler(APIv1DuplicateHandler)
	Handlers["/api/v1/suggesttags"] = makeStatHandler(APIv1SuggestTagsHandler)
	Handlers["/api/v1/similar"] = makeStatHandler(APIv1SimilarPostsHandler)

	Handlers["/captcha/"] = makeStatHandler(captcha.Server(150, 64).ServeHTTP)
	//Handlers["/verify/"] = verifyCaptcha
	Handlers["/test/"] = testHandler
	Handlers["/root/"] = rootHandler

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
			if len(p.Name()) >= 5 && p.Name()[len(p.Name())-5:] == ".html" {
				names = append(names, path+"/"+p.Name())
			}
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
