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
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	mm "github.com/kycklingar/MinMax"
	idir "github.com/kycklingar/PBooru/DataManager/ipfs-dirgen"
	paginate "github.com/kycklingar/PBooru/handlers/paginator"
)

const archiveVersion = "0.1"

func (pc *PostCollector) ArchiveSearch() (*archive, error) {
	archivesInProgress.l.Lock()
	defer archivesInProgress.l.Unlock()

	id := fmt.Sprintf("%x", sha256.Sum256([]byte(pc.countIDStr())))
	// Check for inprogress
	if arch, ok := archivesInProgress.m[id]; ok {
		return arch, nil
	}

	// Check for existing, recent archive

	// Create a new archive
	_, err := pc.Search2(1, 0)
	if err != nil {
		return nil, err
	}

	a := &archive{pc: pc, ID: id}
	archivesInProgress.m[id] = a
	go a.start()
	return a, nil
}

func Archive(id string) *archive {
	archivesInProgress.l.Lock()
	defer archivesInProgress.l.Unlock()
	return archivesInProgress.m[id]
}

var archivesInProgress = struct {
	m map[string]*archive
	l sync.Mutex
}{
	m: make(map[string]*archive),
}

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

	percentage := func(c, max int, state archiveState) float32 {
		step := float32(1) / float32(3)

		s := float32(int(state)) / float32(3)

		p := ((float32(c)/float32(max))*step + s)
		return p
	}

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

		arch.percent = percentage(arch.offset, arch.pc.TotalPosts, arch.state)
		time.Sleep(time.Millisecond * 200)
	}

	var (
		afiles []archiveFile
		w      strings.Builder
	)

	arch.state = stateCreate
	for i, p := range posts {
		// Create the post file
		pdir, fname := path.Split(p.FilePath)
		afiles = append(afiles, archiveFile{
			dir:      strings.Split(pdir, "/"),
			filename: fname,
			cid:      p.Post.Hash,
		})

		//Create the thumbnail file if exists
		if thumb := p.Post.ClosestThumbnail(256); thumb != "" {
			pdir, fname = path.Split(p.ThumbnailPath)
			afiles = append(afiles, archiveFile{
				dir:      strings.Split(pdir, "/"),
				filename: fname,
				cid:      thumb,
			})
		}

		// Create post html page
		w.Reset()
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

		arch.percent = percentage(i, len(posts), arch.state)
	}

	var ppp = 50

	// Reverse posts
	for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
		posts[i], posts[j] = posts[j], posts[i]
	}

	for i := 0; i < len(posts); i += ppp {
		var p = struct {
			Posts []archivePost
			Pag   paginate.Paginator
		}{
			Posts: posts[i:mm.Min(len(posts), i+ppp)],
			Pag: paginate.New(
				1+i/ppp,
				len(posts),
				ppp,
				20,
				"./%d",
			),
		}

		w.Reset()
		err := archiveTemplates.ExecuteTemplate(&w, "list", p)
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
			filename: strconv.Itoa(1 + i/ppp),
			cid:      cid,
		})
	}

	w.Reset()
	err := archiveTemplates.ExecuteTemplate(&w, "index", struct {
		Version string
		Ident   archiveIdentity
	}{
		Version: archiveVersion,
		Ident:   arch.searchIdentity(),
	})
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
		dir:      []string{},
		filename: "index.html",
		cid:      cid,
	})

	cid, err = ipfs.Add(
		strings.NewReader(templateCSS),
		shell.CidVersion(1),
		shell.Pin(false),
	)
	if err != nil {
		arch.error(err)
		return
	}

	afiles = append(afiles, archiveFile{
		dir:      []string{},
		filename: "style.css",
		cid:      cid,
	})

	arch.state = stateGenerate
	var count int

	odir, err := archiver(
		idir.NewDir(""),
		func(archiveFile) {
			count++
			arch.percent = percentage(count, len(afiles), arch.state)
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

type archiveIdentity struct {
	And      []*Tag
	Or       []*Tag
	Filter   []*Tag
	Unless   []*Tag
	Mimes    []string
	Altgroup int
}

func (a *archive) searchIdentity() archiveIdentity {
	tags := func(ids []int) []*Tag {
		tags := make([]*Tag, len(ids))
		for i := 0; i < len(ids); i++ {
			tags[i] = NewTag()
			tags[i].ID = ids[i]
			err := tags[i].QueryAll(DB)
			if err != nil {
				log.Println(err)
				return nil
			}
		}
		return tags
	}

	ai := archiveIdentity{
		And:      tags(a.pc.id),
		Or:       tags(a.pc.or),
		Filter:   tags(a.pc.filter),
		Unless:   tags(a.pc.unless),
		Altgroup: a.pc.altGroup,
	}

	for _, m := range a.pc.mimeIDs {
		for _, mime := range Mimes {
			if mime.ID == m {
				ai.Mimes = append(ai.Mimes, mime.String())
			}
		}
	}

	return ai
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
