package forum

type Post struct {
	Thread int
	Id     int
	Title  string
	Body   Body
	//Poster *DM.User
	//Created DM.timestamp
	Backlinks []int
}
