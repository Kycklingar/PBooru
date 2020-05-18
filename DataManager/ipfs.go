package DataManager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

var ipfsAPI = "http://localhost:5001/api/v0/"

// func GetIpfsPageFromSearch(tagString string) (string, error) {
// 	// posts, totalPosts, err := GetPosts(tagString, true, 100000000, 0)
// 	// if err != nil {
// 	// 	return "", err
// 	// }
// 	//fmt.Println(ipfsAPI)

// 	var pc PostCollector
// 	var err error
// 	if err = pc.Get(tagString, false, 250, 0); err != nil {
// 		log.Print(err)
// 		return "", err
// 	}

// 	if pc.TotalPosts <= 0 {
// 		log.Print("No posts")
// 		return "", nil
// 	}

// 	var fileHash = ""
// 	//var tagsHash = ""
// 	fmt.Println(pc.TotalPosts)
// 	for i := 0; i*250 < pc.TotalPosts; i++ {
// 		fmt.Println(i)
// 		for _, post := range pc.GetW(250, i*250) {
// 			//fmt.Println("Adding ", post.Hash())
// 			prevHash := fileHash
// 			fileHash, err = ipfsPatchLink(fileHash, strconv.Itoa(post.ID()), post.Hash())
// 			// IPFS has error, zd2 hashes won't add to dirs
// 			if err != nil {
// 				log.Println("Err:", err, fileHash)
// 				fileHash = prevHash
// 				continue
// 			}
// 			//fmt.Println("Got", fileHash)
// 		}
// 	}

// 	return fileHash, nil
// }

func ipfsAdd(file io.Reader) (string, error) {
	bodyBuff := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuff)

	fileWriter, err := bodyWriter.CreateFormFile("arg", "")
	if err != nil {
		return "", err
	}

	_, err = io.Copy(fileWriter, file)
	if err != nil {
		return "", err
	}

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post(ipfsAPI+"add?cid-version=1&fscache&pin=false", contentType, bodyBuff)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New(resp.Status)
	}

	body := &bytes.Buffer{}
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}

	f := make(map[string]interface{})

	json.Unmarshal(body.Bytes(), &f)

	m, ok := f["Hash"].(string)
	if !ok {
		return "", errors.New("wrong type")
	}

	return m, nil
}

func mfsCP(dir, mhash string, flush bool) error {
	directory := dir + mhash[len(mhash)-2:] + "/"
	var err error
	if mfsExists(directory) != nil {
		if err := mfsMkdir(directory); err != nil {
			log.Println(err)
			return err
		}
	} else if err = mfsExists(directory + mhash); err == nil {
		return nil
	} else if err.Error() == "no entries" {
		return err
	}

	var fl string
	if !flush {
		fl = "flush=false&"
	}

	uri := fmt.Sprintf("%s/files/cp?%sarg=%s&arg=%s", ipfsAPI, fl, "/ipfs/"+mhash, directory+mhash)
	//fmt.Println(uri)
	res, err := http.PostForm(uri, nil)
	if err != nil {
		log.Println(err)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		b := bytes.Buffer{}
		b.ReadFrom(res.Body)
		return errors.New(fmt.Sprint(res.Status, string(b.Bytes())))
	}

	return nil
}

func mfsMkdir(dir string) error {
	res, err := http.PostForm(ipfsAPI + "files/mkdir?parents=true&arg=" + dir, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return errors.New(res.Status)
	}

	return nil
}

func mfsExists(dir string) error {
	res, err := http.PostForm(ipfsAPI + "files/ls?arg=" + dir, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	b := bytes.Buffer{}
	b.ReadFrom(res.Body)

	if res.StatusCode != 200 {
		return errors.New(fmt.Sprintf("HTTP Error %d, %s", res.StatusCode, string(b.Bytes())))
	}

	v := make(map[string]interface{})
	json.Unmarshal(b.Bytes(), &v)

	if v["Type"] == "error" {
		return errors.New(fmt.Sprint("Path doesn't exist")) //"File doesn't match hash:", v["Hash"], hash))
	}
	if _, ok := v["Entries"]; !ok {
		return errors.New("no entries")
	}

	return nil
}

func mfsFlush(dir string) error {
	res, err := http.PostForm(ipfsAPI + "/files/flush?arg=" + dir, nil)
	if err != nil {
		log.Println(err)
		return err
	}

	res.Body.Close()
	return nil
}

func ipfsPatchLink(rootHash, name, linkHash string) (string, error) {
	if rootHash == "" {
		resp, err := http.PostForm(
			ipfsAPI+"object/new?arg=unixfs-dir",
			nil,
		)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body := &bytes.Buffer{}
		_, err = body.ReadFrom(resp.Body)
		if err != nil {
			return "", err
		}

		f := make(map[string]interface{})

		json.Unmarshal(body.Bytes(), &f)

		m, ok := f["Hash"].(string)
		if !ok {
			return "", errors.New("wrong type")
		}
		rootHash = m
	}

	resp, err := http.PostForm(ipfsAPI + fmt.Sprintf("object/patch/add-link?arg=%s&arg=%s&arg=%s", rootHash, name, linkHash), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body := &bytes.Buffer{}
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}

	f := make(map[string]interface{})

	json.Unmarshal(body.Bytes(), &f)

	m, ok := f["Hash"].(string)
	if !ok {
		return "", errors.New("wrong type")
	}

	return m, nil
}

func ipfsCat(hash string) io.ReadCloser {
	if len(hash) < 46 {
		return nil
	}

	res, err := http.PostForm(
		fmt.Sprintf(ipfsAPI+"cat?arg=%s", hash),
		nil,
	)
	if err != nil {
		log.Println(err)
		return nil
	}

	if res.StatusCode != 200 {
		readError(res)
		return nil
	}

	return res.Body
}

func readError(res *http.Response) {
	var b = make([]byte, res.ContentLength)
	res.Body.Read(b)
	fmt.Println(string(b))
	res.Body.Close()
}

func ipfsSize(hash string) int64 {
	if len(hash) < 46 {
		return 0
	}

	res, err := http.PostForm(
		fmt.Sprintf(ipfsAPI+"cat?arg=%s", hash),
		nil,
	)
	if err != nil {
		log.Println(err)
		return 0
	}

	if res.StatusCode != 200 {
		readError(res)
		return 0
	}
	res.Body.Close()

	sizeStr := res.Header.Get("X-Content-Length")

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		log.Println(err)
		return 0
	}

	return size
}

func ipfsUpgradeCidBase32(hash string) (string, error) {
	cl := http.Client{}
	res, err := cl.PostForm(
		ipfsAPI+"cid/base32",
		url.Values{
			"arg": {hash},
		},
	)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer res.Body.Close()

	b := bytes.Buffer{}
	b.ReadFrom(res.Body)

	if res.StatusCode != 200 {
		return "", fmt.Errorf("HTTP Status %d: %s", res.StatusCode, string(b.Bytes()))
	}

	var m struct {
		CidStr    string
		Formatted string
		ErrorMsg  string
	}

	json.Unmarshal(b.Bytes(), &m)

	if m.ErrorMsg != "" {
		return "", fmt.Errorf("IPFS Error: %s", m.ErrorMsg)
	}

	if m.Formatted == "" {
		return "", fmt.Errorf("No formatted hash returned")
	}

	return m.Formatted, nil
}
