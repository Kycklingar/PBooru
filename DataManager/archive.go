package DataManager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"sync"

	shell "github.com/ipfs/go-ipfs-api"
	idir "github.com/kycklingar/PBooru/DataManager/ipfs-dirgen"
)

const archiveVersion = 1

func (pc *PostCollector) ArchiveSearch() (*archive, error) {
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(pc.countIDStr())))
	// Check for inprogress
	if arch, ok := archivesInProgress[id]; ok {
		return arch, nil
	}

	// Check for existing, recent archive

	// Create a new archive
	_, err := pc.Search2(1, 0)
	if err != nil {
		return nil, err
	}

	a := &archive{pc: pc, ID: id}
	archivesInProgress[id] = a
	go a.start()
	return a, nil
}

func Archive(id string) *archive {
	return archivesInProgress[id]
}

var archivesInProgress = make(map[string]*archive)

type archiveState int

const (
	stateGather archiveState = iota
	stateCreate
	stateGenerate
	statePut
	stateComplete
	stateError
)

type archive struct {
	ID string

	pc     *PostCollector
	offset int
	err    error

	percent float32
	state   archiveState

	Cid string
}

type ProgressState struct {
	Message string
	Percent float32
}

func (arch *archive) Progress() ProgressState {
	var pstate = ProgressState{
		Percent: arch.percent * 100,
	}

	switch arch.state {
	case stateGather:
		pstate.Message = "Gathering posts from database"
	case stateCreate:
		pstate.Message = "Creating metadata files"
	case stateGenerate:
		pstate.Message = "Generating directory structure"
	case statePut:
		pstate.Message = "Putting directory on IPFS"
	case stateComplete:
		pstate.Message = "All done"
	case stateError:
		pstate.Message = "An irrecoverable error has occured\n" + arch.err.Error()
	}

	return pstate
}

func (arch *archive) error(err error) {
	arch.state = stateError
	arch.err = err
}

func (arch *archive) start() {
	type archivePost struct {
		Post          *Post
		Tags          []*Tag
		FilePath      string
		ThumbnailPath string
		PostPath      string
	}

	var posts []archivePost

	arch.state = stateGather
	for ; arch.offset < arch.pc.TotalPosts; arch.offset += 100 {
		results, err := arch.pc.Search2(100, arch.offset)
		if err != nil {
			arch.error(err)
			return
		}

		for _, res := range results {
			err = res.Post.QMul(
				DB,
				PFHash,
				PFThumbnails,
				PFMime,
			)
			if err != nil {
				arch.error(err)
				return
			}

			var thumbPath string
			if thumb := res.Post.ClosestThumbnail(256); thumb != "" {
				thumbPath = path.Join("thumbnails", cidDir(res.Post.Hash), res.Post.Hash)
			}

			var a = archivePost{
				Post:          res.Post,
				Tags:          res.Tags,
				FilePath:      storeFileDest(res.Post.Hash),
				ThumbnailPath: thumbPath,
				PostPath:      path.Join("posts", cidDir(res.Post.Hash), res.Post.Hash),
			}

			posts = append(posts, a)
		}

		arch.percent = (float32(arch.offset) / float32(arch.pc.TotalPosts)) * 0.50
	}

	var afiles []archiveFile

	arch.state = stateCreate
	for i, p := range posts {
		// Create the post file
		pdir, fname := path.Split(p.FilePath)
		afiles = append(afiles, archiveFile{
			dir:      strings.Split(pdir, "/"),
			filename: fname,
			cid:      p.Post.Hash,
		})

		// Create the thumbnail file if exists
		if thumb := p.Post.ClosestThumbnail(256); thumb != "" {
			pdir, fname = path.Split(p.ThumbnailPath)
			afiles = append(afiles, archiveFile{
				dir:      strings.Split(pdir, "/"),
				filename: fname,
				cid:      thumb,
			})
		}

		// Create post html page
		var w strings.Builder
		err := archiveTemplates.ExecuteTemplate(&w, "post", p)
		if err != nil {
			arch.error(err)
			return
		}

		cid, err := ipfs.Add(
			strings.NewReader(w.String()),
			shell.CidVersion(1),
			shell.Pin(false),
		)
		if err != nil {
			arch.error(err)
			return
		}
		pdir, fname = path.Split(p.PostPath)
		afiles = append(afiles, archiveFile{
			dir:      strings.Split(pdir, "/"),
			filename: fname,
			cid:      cid,
		})

		arch.percent = (float32(i) / float32(len(posts)) * 0.40) + 0.50
	}

	for i := 0; i < len(posts); i += 50 {
		sl := posts[i:max(len(posts)-1, i+50)]

		var w strings.Builder
		err := archiveTemplates.ExecuteTemplate(&w, "list", sl)
		if err != nil {
			arch.error(err)
			return
		}

		cid, err := ipfs.Add(
			strings.NewReader(w.String()),
			shell.CidVersion(1),
			shell.Pin(false),
		)
		if err != nil {
			arch.error(err)
			return
		}
		afiles = append(afiles, archiveFile{
			dir:      []string{"list"},
			filename: strconv.Itoa(1 + i/50),
			cid:      cid,
		})
	}

	arch.state = stateGenerate
	var count int

	odir, err := archiver(
		idir.NewDir(""),
		func(archiveFile) {
			count++
			arch.percent = (float32(count)/float32(len(afiles)))*0.10 + 0.90
		},
		afiles...,
	)
	if err != nil {
		arch.error(err)
		return
	}

	arch.state = statePut
	arch.Cid, _, err = odir.Put(ipfs)
	if err != nil {
		arch.error(err)
		return
	}

	arch.percent = 1.0
	arch.state = stateComplete

	// Evacuate archive aip
}

type archiveFile struct {
	dir      []string
	filename string
	size     uint64
	cid      string
}

func archiver(dir *idir.Dir, report func(archiveFile), files ...archiveFile) (*idir.Dir, error) {
	var (
		looperr error
		in      = make(chan archiveFile, 10)
		out     = make(chan archiveFile)
		done    = make(chan bool)
		wg      sync.WaitGroup
	)

	statloop := func(wg *sync.WaitGroup, in, out chan archiveFile) {
		defer wg.Done()
		for f := range in {
			stat, err := ipfs.FilesStat(context.Background(), "/ipfs/"+f.cid)
			if err != nil {
				looperr = err
				return
			}
			f.size = stat.Size
			out <- f
		}
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go statloop(&wg, in, out)
	}

	go func() {
		for _, f := range files {
			if looperr != nil {
				close(in)
			}

			in <- f
		}

		close(in)
	}()

	go func(done chan bool) {
		for {
			select {
			case <-done:
				done <- true
				return
			case f := <-out:
				report(f)
				d := dir.AddDirs(f.dir...)
				err := d.AddLink(f.filename, f.cid, f.size)
				// Should never error
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}(done)

	wg.Wait()
	done <- true
	<-done

	return dir, looperr
}
