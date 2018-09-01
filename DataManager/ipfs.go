package DataManager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	resp, err := http.Post(ipfsAPI+"add?cid-version=1&fscache", contentType, bodyBuff)
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
		fmt.Println(err)
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
