package handlers

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/dchest/captcha"
)

type Config struct {
	IPFSDaemonMap map[string]string
}

func (c Config) Default() {
	if c.IPFSDaemonMap == nil {
		c.IPFSDaemonMap = make(map[string]string)
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

func init() {
	Templates = template.New("")
	Templates.Funcs(template.FuncMap{
		"noescape":  func(x string) template.HTML { return template.HTML(x) },
		"urlEncode": UrlEncode,
		"urlDecode": UrlDecode})

	tmpls, err := loadTemplates("./templates/")
	if err != nil {
		log.Fatal(err)
	}
	if _, err = Templates.ParseFiles(tmpls...); err != nil {
		log.Fatal(err)
	}

	Handlers = make(map[string]func(http.ResponseWriter, *http.Request))
	//Handlers = make(map[string]http.Handler)

	Handlers["/"] = IndexHandler
	Handlers["/post/"] = PostHandler
	Handlers["/posts"] = PostsHandler
	Handlers["/posts/"] = PostsHandler
	Handlers["/upload"] = UploadHandler
	Handlers["/upload/"] = UploadHandler
	Handlers["/options"] = OptionsHandler
	Handlers["/options/"] = OptionsHandler
	Handlers["/tags/"] = TagsHandler
	Handlers["/taghistory/"] = TagHistoryHandler
	Handlers["/login/"] = LoginHandler
	Handlers["/logout/"] = LogoutHandler
	Handlers["/register/"] = RegisterHandler
	Handlers["/deletepost"] = RemovePostHandler
	Handlers["/wall/"] = CommentWallHandler
	Handlers["/comics/"] = ComicsHandler
	Handlers["/comic/"] = ComicHandler
	Handlers["/comic/add"] = ComicAddHandler
	Handlers["/comic/edit"] = EditComicHandler
	Handlers["/links/"] = func(w http.ResponseWriter, r *http.Request) { renderTemplate(w, "links", nil) }

	Handlers["/dups/add/"] = NewDuplicateHandler
	Handlers["/admin"] = func(w http.ResponseWriter, r *http.Request) {
		user, info := getUser(w, r)
		p := struct {
			UserInfo UserInfo
			User     User
		}{info, tUser(user)}

		renderTemplate(w, "admin", p)
	}
	Handlers["/slideshow"] = func(w http.ResponseWriter, r *http.Request) {
		_, ui := getUser(w, r)
		renderTemplate(w, "slideshow", ui)
	}

	Handlers["/similar/"] = findSimilarHandler

	Handlers["/api/"] = APIHandler
	Handlers["/api/v1/"] = APIv1Handler
	Handlers["/api/v1/post"] = APIv1PostHandler
	Handlers["/api/v1/posts"] = APIv1PostsHandler
	Handlers["/api/v1/duplicate"] = APIv1DuplicateHandler
	Handlers["/api/v1/suggesttags"] = APIv1SuggestTagsHandler
	Handlers["/api/v1/similar"] = APIv1SimilarPostsHandler

	Handlers["/captcha/"] = captcha.Server(150, 64).ServeHTTP
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
