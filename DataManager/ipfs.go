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

	if mfsExists(directory) != nil {
		if err := mfsMkdir(directory); err != nil {
			log.Println(err)
			return err
		}
	} else if mfsExists(directory+mhash) == nil {
		return nil
	}

	var fl string
	if !flush {
		fl = "flush=false&"
	}

	uri := fmt.Sprintf("%s/files/cp?%sarg=%s&arg=%s", ipfsAPI, fl, "/ipfs/"+mhash, directory+mhash)
	//fmt.Println(uri)
	res, err := http.Get(uri)
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
	res, err := http.Get(ipfsAPI + "files/mkdir?parents=true&arg=" + dir)
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
	res, err := http.Get(ipfsAPI + "files/ls?arg=" + dir)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	b := bytes.Buffer{}
	b.ReadFrom(res.Body)
	v := make(map[string]interface{})
	json.Unmarshal(b.Bytes(), &v)

	if v["Type"] == "error" {
		return errors.New(fmt.Sprint("Path doesn't exist")) //"File doesn't match hash:", v["Hash"], hash))
	}
	return nil
}

func mfsFlush(dir string) error {
	res, err := http.Get(ipfsAPI + "/files/flush?arg=" + dir)
	if err != nil {
		log.Println(err)
		return err
	}

	res.Body.Close()
	return nil
}

func ipfsPatchLink(rootHash, name, linkHash string) (string, error) {
	hc := http.Client{}

	if rootHash == "" {
		resp, err := hc.Get(ipfsAPI + "object/new?arg=unixfs-dir")
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

	resp, err := hc.Get(ipfsAPI + fmt.Sprintf("object/patch/add-link?arg=%s&arg=%s&arg=%s", rootHash, name, linkHash))
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
	cl := http.Client{}
	res, err := cl.Get(fmt.Sprintf(ipfsAPI+"cat?arg=%s", hash))
	if err != nil {
		log.Println(err)
		return nil
	}

	if res.StatusCode != 200 {
		b := make([]byte, 10)
		res.Body.Read(b)
		fmt.Println(string(b))
		res.Body.Close()
		return nil
	}

	return res.Body
}

func ipfsSize(hash string) int64{
	if len(hash) < 46 {
		return 0
	}

	cl := http.Client{}
	res, err := cl.Get(fmt.Sprintf(ipfsAPI+"object/stat?arg=%s", hash))
	if err != nil{
		log.Println(err)
		return 0
	}
	defer res.Body.Close()

	if res.StatusCode != 200{
		log.Println("status ", res.StatusCode)
		return 0
	}


	body := &bytes.Buffer{}
	_, err = body.ReadFrom(res.Body)
	if err != nil {
		log.Println(err)
		return 0
	}

	f := make(map[string]interface{})

	json.Unmarshal(body.Bytes(), &f)
	fmt.Println(f)

	m, ok := f["CumulativeSize"]
	if !ok {
		log.Println("not ok")
		return 0
	}

	//size, err := strconv.ParseInt(m.(string), 10, 64)
	//if err != nil{
	//	log.Println(err)
	//	return 0
	//}
	return int64(m.(float64))
}
