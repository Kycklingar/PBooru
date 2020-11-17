package DataManager

import (
	"context"
	"io"

	shell "github.com/ipfs/go-ipfs-api"
)

//var ipfsAPI = "http://localhost:5001/api/v0/"
var ipfs *shell.Shell

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

func ipfsCat(hash string) (io.ReadCloser, error) {
	return ipfs.Cat(hash)
}

func ipfsSize(hash string) (int64, error) {
	links, err := ipfs.List(hash)
	if err != nil {
		return 0, err
	}

	var sum int64
	if len(links) > 0 {
		for _, link := range links {
			sum += int64(link.Size)
		}
	} else {
		stat, err := ipfs.ObjectStat(hash)
		if err != nil {
			return 0, err
		}
		sum = int64(stat.CumulativeSize)
	}

	return sum, nil
}

func ipfsUpgradeCidBase32(hash string) (string, error) {
	var out string
	err := ipfs.Request("cid/base32", hash).Exec(context.Background(), &out)

	return out, err
}
