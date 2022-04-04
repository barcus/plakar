/*
 * Copyright (c) 2021 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package ui

import (
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"math/rand"
	"mime"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/poolpOrg/plakar/filesystem"
	"github.com/poolpOrg/plakar/snapshot"
	"github.com/poolpOrg/plakar/storage"

	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

var lstore *storage.Store

//go:embed base.tmpl
var baseTemplate string

//go:embed store.tmpl
var storeTemplate string

//go:embed browse.tmpl
var browseTemplate string

//go:embed object.tmpl
var objectTemplate string

//go:embed search.tmpl
var searchTemplate string

var templates map[string]*template.Template

type SnapshotSummary struct {
	Uuid         string
	CreationTime time.Time
	Version      string
	Hostname     string
	Username     string
	CommandLine  string

	Roots       uint64
	Directories uint64
	Files       uint64
	NonRegular  uint64
	Pathnames   uint64
	Objects     uint64
	Chunks      uint64

	Size uint64
}

func getSnapshots(store *storage.Store) ([]*snapshot.Snapshot, error) {
	snapshotsList, err := snapshot.List(store)
	if err != nil {
		return nil, err
	}

	result := make([]*snapshot.Snapshot, 0)

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	for _, snapshotUuid := range snapshotsList {
		wg.Add(1)
		go func(snapshotUuid string) {
			defer wg.Done()
			snapshotInstance, err := snapshot.Load(store, snapshotUuid)
			if err != nil {
				return
			}
			mu.Lock()
			result = append(result, snapshotInstance)
			mu.Unlock()
		}(snapshotUuid)
	}
	wg.Wait()

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreationTime.Before(result[j].CreationTime)
	})

	return result, nil
}

func (summary *SnapshotSummary) HumanSize() string {
	return humanize.Bytes(summary.Size)
}

func SnapshotToSummary(snapshot *snapshot.Snapshot) *SnapshotSummary {
	ss := &SnapshotSummary{}
	ss.Uuid = snapshot.Uuid
	ss.CreationTime = snapshot.CreationTime
	ss.Version = snapshot.Version
	ss.Hostname = snapshot.Hostname
	ss.Username = snapshot.Username
	ss.CommandLine = snapshot.CommandLine
	ss.Roots = uint64(len(snapshot.Filesystem.ScannedDirectories))
	ss.Directories = uint64(len(snapshot.Filesystem.Directories))
	ss.Files = uint64(len(snapshot.Filesystem.Files))
	ss.NonRegular = uint64(len(snapshot.Filesystem.NonRegular))
	ss.Pathnames = uint64(len(snapshot.Pathnames))
	ss.Objects = uint64(len(snapshot.Objects))
	ss.Chunks = uint64(len(snapshot.Chunks))
	ss.Size = snapshot.Size
	return ss
}

func viewStore(w http.ResponseWriter, r *http.Request) {

	snapshotsList, _ := getSnapshots(lstore)

	mimeTypes := make(map[string]uint64)
	majorTypes := make(map[string]uint64)
	extensions := make(map[string]uint64)
	totalFiles := uint64(0)

	res := make([]*SnapshotSummary, 0)
	for _, snap := range snapshotsList {
		res = append(res, SnapshotToSummary(snap))

		for key, value := range snap.ContentTypeToObjects {
			contentType := strings.Split(key, ";")[0]
			contentMajorType := strings.Split(key, "/")[0]
			if contentType == "" {
				contentType = "unknown"
				contentMajorType = "unknown"
			}
			for _, _ = range value {
				if _, exists := mimeTypes[contentType]; !exists {
					mimeTypes[contentType] = 0
				}
				if _, exists := majorTypes[contentMajorType]; !exists {
					majorTypes[contentMajorType] = 0
				}
				mimeTypes[contentType]++
				majorTypes[contentMajorType]++
			}
		}

		for key := range snap.Pathnames {
			ext := strings.ToLower(filepath.Ext(key))
			if ext == "" {
				ext = "none"
			}
			if _, exists := extensions[ext]; !exists {
				extensions[ext] = 0
			}
			extensions[ext]++
			totalFiles++
		}
	}

	mimeTypesPct := make(map[string]float64)
	majorTypesPct := make(map[string]float64)
	extensionsPct := make(map[string]float64)

	for key, value := range mimeTypes {
		mimeTypesPct[key] = math.Round((float64(value)/float64(totalFiles)*100)*100) / 100
	}
	for key, value := range majorTypes {
		majorTypesPct[key] = math.Round((float64(value)/float64(totalFiles)*100)*100) / 100
	}
	for key, value := range extensions {
		extensionsPct[key] = math.Round((float64(value)/float64(totalFiles)*100)*100) / 100
	}

	ctx := &struct {
		Store         storage.StoreConfig
		Snapshots     []*SnapshotSummary
		MajorTypes    map[string]uint64
		MimeTypes     map[string]uint64
		Extensions    map[string]uint64
		MajorTypesPct map[string]float64
		MimeTypesPct  map[string]float64
		ExtensionsPct map[string]float64
	}{
		lstore.Configuration(),
		res,
		majorTypes,
		mimeTypes,
		extensions,
		majorTypesPct,
		mimeTypesPct,
		extensionsPct,
	}

	templates["store"].Execute(w, ctx)
}

func browse(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["snapshot"]
	path := vars["path"]

	snap, err := snapshot.Load(lstore, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if path == "" {
		path = "/"
	}

	_, exists := snap.LookupInodeForPathname(path)
	if !exists {
		http.Error(w, "", http.StatusNotFound)
		return

	}

	directories := make([]*filesystem.Fileinfo, 0)
	files := make([]*filesystem.Fileinfo, 0)
	symlinks := make([]*filesystem.Fileinfo, 0)
	symlinksResolve := make(map[string]string)
	others := make([]*filesystem.Fileinfo, 0)

	children, _ := snap.LookupPathChildren(path)
	for _, fileinfo := range children {
		if fileinfo.Mode.IsDir() {
			directories = append(directories, fileinfo)
		} else if fileinfo.Mode.IsRegular() {
			files = append(files, fileinfo)
		} else {
			pathname := fmt.Sprintf("%s/%s", path, fileinfo.Name)
			if _, exists := snap.Filesystem.Symlinks[pathname]; exists {
				symlinks = append(symlinks, fileinfo)
				symlinksResolve[fileinfo.Name] = snap.Filesystem.Symlinks[pathname]
			} else {
				others = append(others, fileinfo)
			}
		}
	}

	sort.Slice(directories, func(i, j int) bool {
		return strings.Compare(directories[i].Name, directories[j].Name) < 0
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.Compare(files[i].Name, files[j].Name) < 0
	})
	sort.Slice(symlinks, func(i, j int) bool {
		return strings.Compare(symlinks[i].Name, symlinks[j].Name) < 0
	})
	sort.Slice(others, func(i, j int) bool {
		return strings.Compare(others[i].Name, others[j].Name) < 0
	})

	nav := make([]string, 0)
	navLinks := make(map[string]string)
	atoms := strings.Split(path, "/")[1:]
	for offset, atom := range atoms {
		nav = append(nav, atom)
		navLinks[atom] = "/" + strings.Join(atoms[:offset+1], "/")
	}

	ctx := &struct {
		Snapshot        *snapshot.Snapshot
		Directories     []*filesystem.Fileinfo
		Files           []*filesystem.Fileinfo
		Symlinks        []*filesystem.Fileinfo
		SymlinksResolve map[string]string
		Others          []*filesystem.Fileinfo
		Path            string
		Scanned         []string
		Navigation      []string
		NavigationLinks map[string]string
	}{snap, directories, files, symlinks, symlinksResolve, others, path, snap.Filesystem.ScannedDirectories, nav, navLinks}
	templates["browse"].Execute(w, ctx)

}

func object(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["snapshot"]
	path := vars["path"]

	snap, err := snapshot.Load(lstore, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	checksum, ok := snap.Pathnames[path]
	if !ok {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	object := snap.Objects[checksum]
	info, _ := snap.LookupInodeForPathname(path)

	chunks := make([]*snapshot.Chunk, 0)
	for _, chunkChecksum := range object.Chunks {
		chunks = append(chunks, snap.Chunks[chunkChecksum])
	}

	root := ""
	for _, atom := range strings.Split(path, "/") {
		root = root + atom + "/"
		if _, ok := snap.Filesystem.LookupInodeForDirectory(root); ok {
			break
		}
	}

	nav := make([]string, 0)
	navLinks := make(map[string]string)
	atoms := strings.Split(path, "/")[1:]
	for offset, atom := range atoms {
		nav = append(nav, atom)
		navLinks[atom] = "/" + strings.Join(atoms[:offset+1], "/")
	}

	enableViewer := false
	if strings.HasPrefix(object.ContentType, "text/") ||
		strings.HasPrefix(object.ContentType, "image/") ||
		strings.HasPrefix(object.ContentType, "audio/") ||
		strings.HasPrefix(object.ContentType, "video/") ||
		object.ContentType == "application/pdf" ||
		object.ContentType == "application/x-tex" {
		enableViewer = true
	}

	ctx := &struct {
		Snapshot        *snapshot.Snapshot
		Object          *snapshot.Object
		Chunks          []*snapshot.Chunk
		Info            *filesystem.Fileinfo
		Root            string
		Path            string
		Navigation      []string
		NavigationLinks map[string]string
		EnableViewer    bool
	}{snap, object, chunks, info, root, path, nav, navLinks, enableViewer}
	templates["object"].Execute(w, ctx)
}

func raw(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["snapshot"]
	path := vars["path"]
	download := r.URL.Query().Get("download")
	highlight := r.URL.Query().Get("highlight")

	snap, err := snapshot.Load(lstore, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	checksum, ok := snap.Pathnames[path]
	if !ok {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	object := snap.Objects[checksum]
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = object.ContentType
	}

	if contentType == "application/x-tex" {
		contentType = "text/plain"
	}

	if !strings.HasPrefix(object.ContentType, "text/") || highlight == "" {
		w.Header().Add("Content-Type", contentType)
		if download != "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(path)))
		}
		for _, chunkChecksum := range object.Chunks {
			data, err := snap.GetChunk(chunkChecksum)
			if err != nil {
			}
			w.Write(data)
		}
		return
	}

	content := []byte("")
	for _, chunkChecksum := range object.Chunks {
		data, err := snap.GetChunk(chunkChecksum)
		if err != nil {
		}
		content = append(content, data...)
	}

	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(string(content))
	}
	if lexer == nil {
		w.Header().Add("Content-Type", contentType)
		w.Write(content)
		return
	}

	w.Header().Add("Content-Type", "text/html")
	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("html")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	iterator, err := lexer.Tokenise(nil, string(content))
	err = formatter.Format(w, style, iterator)
	if err != nil {
		w.Header().Add("Content-Type", contentType)
		w.Write(content)
	}
	return
}

func search_snapshots(w http.ResponseWriter, r *http.Request) {
	urlParams := r.URL.Query()
	q := urlParams["q"][0]
	queryKind := urlParams["kind"]
	queryMime := urlParams["mime"]
	queryExt := urlParams["ext"]

	kind := ""
	if queryKind != nil {
		kind = queryKind[0]
	} else {
		kind = ""
	}
	mime := ""
	if queryMime != nil {
		mime = queryMime[0]
	} else {
		mime = ""
	}
	ext := ""
	if queryExt != nil {
		ext = queryExt[0]
	} else {
		ext = ""
	}

	snapshots, err := snapshot.List(lstore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	snapshotsList := make([]*snapshot.Snapshot, 0)
	for _, id := range snapshots {
		snapshot, err := snapshot.Load(lstore, id)
		if err != nil {
			/* failed to lookup snapshot */
			continue
		}
		snapshotsList = append(snapshotsList, snapshot)
	}
	sort.Slice(snapshotsList, func(i, j int) bool {
		return snapshotsList[i].CreationTime.Before(snapshotsList[j].CreationTime)
	})

	directories := make([]struct {
		Snapshot string
		Date     string
		Path     string
	}, 0)
	files := make([]struct {
		Snapshot string
		Date     string
		Path     string
	}, 0)
	for _, snap := range snapshotsList {
		if kind == "" && mime == "" && ext == "" {
			for _, directory := range snap.Filesystem.ListDirectories() {
				if strings.Contains(directory, q) {
					directories = append(directories, struct {
						Snapshot string
						Date     string
						Path     string
					}{snap.Uuid, snap.CreationTime.String(), directory})
				}
			}
		}
		for file, checksum := range snap.Pathnames {
			if strings.Contains(file, q) {
				object := snap.Objects[checksum]
				if kind != "" && !strings.HasPrefix(object.ContentType, kind+"/") {
					continue
				}
				if mime != "" && !strings.HasPrefix(object.ContentType, mime) {
					continue
				}
				if ext != "" && filepath.Ext(file) != ext {
					continue
				}

				files = append(files, struct {
					Snapshot string
					Date     string
					Path     string
				}{snap.Uuid, snap.CreationTime.String(), file})
			}
		}
	}
	sort.Slice(directories, func(i, j int) bool {
		return directories[i].Date < directories[j].Date && strings.Compare(directories[i].Path, directories[j].Path) < 0
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Date < files[j].Date && strings.Compare(files[i].Path, files[j].Path) < 0
	})

	ctx := &struct {
		SearchTerms string
		Directories []struct {
			Snapshot string
			Date     string
			Path     string
		}
		Files []struct {
			Snapshot string
			Date     string
			Path     string
		}
	}{q, directories, files}
	templates["search"].Execute(w, ctx)
}

func Ui(store *storage.Store, spawn bool) {
	lstore = store

	templates = make(map[string]*template.Template)

	t, err := template.New("store").Parse(baseTemplate + storeTemplate)
	if err != nil {
		panic(err)
	}
	templates[t.Name()] = t

	t, err = template.New("browse").Parse(baseTemplate + browseTemplate)
	if err != nil {
		panic(err)
	}
	templates[t.Name()] = t

	t, err = template.New("object").Parse(baseTemplate + objectTemplate)
	if err != nil {
		panic(err)
	}
	templates[t.Name()] = t

	t, err = template.New("search").Parse(baseTemplate + searchTemplate)
	if err != nil {
		panic(err)
	}
	templates[t.Name()] = t

	port := rand.Uint32() % 0xffff
	url := fmt.Sprintf("http://localhost:%d", port)

	fmt.Println("lauching UI at", url)
	if spawn {
		switch runtime.GOOS {
		case "linux":
			err = exec.Command("xdg-open", url).Start()
		case "windows":
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		case "darwin":
			err = exec.Command("open", url).Start()
		default:
			err = fmt.Errorf("unsupported platform")
		}
		_ = err
	}

	r := mux.NewRouter()
	r.HandleFunc("/", viewStore)
	r.HandleFunc("/snapshot/{snapshot}:/", browse)
	r.HandleFunc("/snapshot/{snapshot}:{path:.+}/", browse)
	r.HandleFunc("/raw/{snapshot}:{path:.+}", raw)
	r.HandleFunc("/snapshot/{snapshot}:{path:.+}", object)

	r.HandleFunc("/search", search_snapshots)

	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}
