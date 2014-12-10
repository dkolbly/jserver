package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/libgit2/git2go"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type User struct {
	Login    string `json:"login"`
	Password string `json:"password"` // MD5 hash of <user>:<realm>:<pass>
}

type Config struct {
	Users []User `json:"users"`
}

func LoadConfig(path string) *Config {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	cfg := &Config{}
	err = json.Unmarshal(buf, cfg)
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}

func main() {
	cfgfile := flag.String("c", "config.json", "config file")
	root := flag.String("root", "/www", "root of tree")
	edit := flag.String("edit", "/edit", "root of edit tree")
	ace := flag.String("ace", "/ace", "root of ace tree")
	listen := flag.String("listen", ":80", "what to listen on")
	flag.Parse()
	cfg := LoadConfig(*cfgfile)

	http.Handle("/edit/ace/",
		http.StripPrefix("/edit/ace/", http.FileServer(http.Dir(*ace))),
	)

	http.Handle("/edit/", &EditServer{
		root:  *root,
		realm: "Jason's Server",
		edit:  http.StripPrefix("/edit/", http.FileServer(http.Dir(*edit))),
		users: cfg.Users,
	})

	http.Handle("/", http.FileServer(http.Dir(*root)))
	log.Fatal(http.ListenAndServe(*listen, nil))
}

type EditServer struct {
	root  string
	users []User
	edit  http.Handler
	realm string
}

func computeNonce() string {
	s := time.Now().Format("%Y-%m-%d %H:%M:%S")
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func md5combine(parts ...string) string {
	h := md5.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{':'})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum([]byte{}))
}

// ServeHTTP processes a request to the "/edit" tree
func (s *EditServer) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
	// First, we check to make sure the user is authenticated
	auth := DigestAuthParams(req)
	if auth == nil ||
		auth["opaque"] != "foo" ||
		auth["algorithm"] != "MD5" ||
		auth["qop"] != "auth" {
		// they did not supply authorization,
		// or it's not the kind we like
		s.NeedAuthorization(rsp, req)
		return
	}
	// Make sure the URI matches
	u, err := url.Parse(auth["uri"])
	if err != nil ||
		req.URL == nil ||
		len(u.Path) > len(req.URL.Path) ||
		!strings.HasPrefix(req.URL.Path, u.Path) {
		s.NeedAuthorization(rsp, req)
		return
	}

	// try all possibilities
	found := false
	for _, u := range s.users {
		ha1 := u.Password
		// Figure out what we are *expecting* them to supply
		ha2 := md5combine(req.Method, req.URL.Path)
		kd := md5combine(
			ha1,
			auth["nonce"],
			auth["nc"],
			auth["cnonce"],
			auth["qop"],
			ha2)
		if auth["response"] == kd {
			found = true
			break
		}
	}
	if !found {
		// hmm, it doesn't match what we expect... try again!
		s.NeedAuthorization(rsp, req)
		return
	}

	fmt.Printf("Handling: %q\n", req.URL.Path)
	fmt.Printf("Method: %s\n", req.Method)

	if req.URL.Path == "/edit/update" && req.Method == "POST" {
		s.HandlePageUpdate(rsp, req)
	} else if req.URL.Path == "/edit/html" && req.Method == "POST" {
		s.HandleHTMLPageEdit(rsp, req)
	} else if strings.HasPrefix(req.URL.Path, "/edit/git/") {
		s.HandleGit(rsp,req)
	} else if strings.HasPrefix(req.URL.Path, "/edit/v/") {
		s.HandleVersions(rsp, req)
	} else if req.URL.Path == "/edit/list" {
		s.HandleListing(rsp, req)
	} else {
		// otherwise, serve the response from the edit tree
		s.edit.ServeHTTP(rsp, req)
	}
}

func gitStatusToString(status git.Status) string {
	switch status {
	case git.StatusCurrent:	return "Current"
	case git.StatusIndexNew:	return "IndexNew"
	case git.StatusIndexModified:	return "IndexModified"
	case git.StatusIndexRenamed:	return "IndexRenamed"
	case git.StatusIndexTypeChange:	return "IndexTypeChange"
	case git.StatusWtNew:	return "WtNew"
	case git.StatusWtModified:	return "WtModified"
	case git.StatusWtDeleted:	return "WtDeleted"
	case git.StatusWtTypeChange:	return "WtTypeChange"
	case git.StatusWtRenamed:	return "WtRenamed"
	case git.StatusIgnored:	return "Ignored"
	}
	return "Unknown"
}

// HandleGit handles all /edit/git commands
func (s *EditServer) HandleGit(rsp http.ResponseWriter, req *http.Request) {
	repo, _ := git.OpenRepository(s.root)
	if strings.HasPrefix(req.URL.Path, "/edit/git/status/") {
		filename := req.URL.Path[17:]
		status, _ := repo.StatusFile(filename)

		rsp.Write([]byte(gitStatusToString(status)))
	}
	rsp.Write([]byte("\n"))
}

// NeedAuthorization tells the browser we need authorization
func (s *EditServer) NeedAuthorization(rsp http.ResponseWriter, req *http.Request) {
	nonce := computeNonce()
	authRequest := fmt.Sprintf(`Digest realm="%s", nonce="%s", qop="auth", opaque="foo", algorithm="MD5"`,
		s.realm,
		nonce,
	)
	fmt.Printf("need: %s\n", authRequest)
	rsp.Header().Set("WWW-Authenticate", authRequest)
	rsp.WriteHeader(http.StatusUnauthorized)
	rsp.Write([]byte("401 Unauthorized\n"))
}

// DigestAuthParams parses the auth parameters into a map
func DigestAuthParams(r *http.Request) map[string]string {
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 || s[0] != "Digest" {
		return nil
	}

	result := map[string]string{}
	for _, kv := range strings.Split(s[1], ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.Trim(parts[0], "\" ")] = strings.Trim(parts[1], "\" ")
	}
	return result
}

// HandlePageUpdate handles a POST to update a page
// (this is the old form page, from /edit/old.hmtl; keeping around
// because it's more general in letting you upload non-HTML like images etc.)
func (s *EditServer) HandlePageUpdate(rsp http.ResponseWriter, req *http.Request) {
	contentType := req.Header["Content-Type"]

	mediatype, params, err := mime.ParseMediaType(contentType[0])
	if err != nil {
		rsp.WriteHeader(http.StatusBadRequest)
		rsp.Write([]byte("error parsing media type"))
		return
	}
	if mediatype != "multipart/form-data" {
		rsp.WriteHeader(http.StatusBadRequest)
		rsp.Write([]byte(fmt.Sprintf("bad media type [outer=%q]", mediatype)))
		return
	}

	rdr := multipart.NewReader(req.Body, params["boundary"])

	var comment []byte
	var content []byte
	var filename string
	ok := true

	for {
		part, err := rdr.NextPart()
		if err != nil {
			break
		}

		v := part.Header.Get("Content-Disposition")
		cdHead, cdParms, err := mime.ParseMediaType(v)
		if err != nil {
			fmt.Printf("Error parsing %q: %s\n", v, err)
			rsp.WriteHeader(http.StatusBadRequest)
			rsp.Write([]byte(fmt.Sprintf("bad media type [inner=%q]", v)))
			return
		}
		fmt.Printf("disposition %q params %v\n", cdHead, cdParms)
		fmt.Printf("Part: %v\n", part.Header)

		fn := part.FormName()
		fmt.Printf("Name: %q\n", fn)
		if fn == "comment" {
			comment, err = ioutil.ReadAll(part)
			if err != nil {
				ok = false
			}
		} else if fn == "page" {
			filename = cdParms["filename"]
			content, err = ioutil.ReadAll(part)
			if err != nil {
				ok = false
			}
		}
	}
	if ok && comment != nil && content != nil {
		filename = path.Base(filename)
		fmt.Printf("%d bytes of comment, %d bytes of %q\n",
			len(comment),
			len(content),
			filename)
		f, err := os.Create(path.Join(s.root, filename))
		if err != nil {
			rsp.WriteHeader(http.StatusInternalServerError)
			rsp.Write([]byte("Error saving file\n"))
			return
		}
		f.Write(content)
		f.Close()
		http.Redirect(rsp, req, "/"+filename, http.StatusMovedPermanently)
	} else {
		rsp.WriteHeader(http.StatusBadRequest)
		rsp.Write([]byte("Error trying to process upload\n"))
	}
}

type FileEntry struct {
	Name     string    `json:"name"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
}

type FileList struct {
	Listing []FileEntry `json:"listing"`
}

type FileVersion struct {
	Hash     string    `json:"hash"`
	Modified time.Time `json:"modified"`
	Comment  string    `json:"comment"`
}

type VersionList struct {
	Listing []FileVersion `json:"listing"`
	Content string        `json:"content"`
}

// HandleVersions returns version history for a file
func (s *EditServer) HandleVersions(rsp http.ResponseWriter, req *http.Request) {
	what := strings.TrimPrefix(req.URL.Path, "/edit/v/")
	src, err := os.Open(path.Join(s.root, what))
	if err != nil {
		log.Printf("Failed to read file: %s", err)
		rsp.WriteHeader(http.StatusBadRequest)
		return
	}
	srcdata, err := ioutil.ReadAll(src)
	if err != nil {
		log.Printf("Failed to read file: %s", err)
		rsp.WriteHeader(http.StatusBadRequest)
		return
	}

	cmd := exec.Command("git", "log", "--format=tformat:%h <%ai> %s", what)
	cmd.Dir = s.root
	buf, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to read git log: %s", err)
		rsp.WriteHeader(http.StatusInternalServerError)
		return
	}

	vlist := &VersionList{
		Content: string(srcdata),
		Listing: []FileVersion{},
	}
	re := regexp.MustCompile("([0-9a-f]+) <([0-9 :-]+)> (.*)\n$")

	rdr := bufio.NewReader(bytes.NewReader(buf))
	for {
		line, err := rdr.ReadString('\n')
		if err != nil {
			break
		}
		log.Printf("==> %q", line)
		submatch := re.FindStringSubmatch(line)
		if submatch != nil {
			log.Printf("  hash %q", submatch[1])
			log.Printf("  date %q", submatch[2])
			log.Printf("  comment %q", submatch[3])
			t, err := time.Parse("2006-01-02 15:04:05 -0700", submatch[2])
			if err == nil {
				vlist.Listing = append(vlist.Listing,
					FileVersion{
						Hash:     submatch[1],
						Modified: t,
						Comment:  submatch[3],
					})
			}
		}
	}
	rspbuf, _ := json.Marshal(vlist)
	rsp.Write(rspbuf)
}

// HandleListing returns a list of all files
func (s *EditServer) HandleListing(rsp http.ResponseWriter, req *http.Request) {
	flist := &FileList{
		Listing: []FileEntry{},
	}

	filepath.Walk(
		s.root,
		func(path string, info os.FileInfo, err error) error {
			if strings.HasSuffix(path, ".git") {
				return filepath.SkipDir
			}
			if !info.IsDir() {
				name := path[len(s.root):]
				flist.Listing = append(
					flist.Listing,
					FileEntry{
						Name:     path[len(s.root)+1:],
						Modified: info.ModTime(),
						Size:     info.Size(),
					},
				)
				log.Printf("walk %q", name)
			}
			return nil
		},
	)
	rspbuf, _ := json.Marshal(flist)
	rsp.Write(rspbuf)
}

// HandlePageUpdate handles a POST to update a page
func (s *EditServer) HandleHTMLPageEdit(rsp http.ResponseWriter, req *http.Request) {
	contentType := req.Header["Content-Type"]

	mediatype, params, err := mime.ParseMediaType(contentType[0])
	if err != nil {
		rsp.WriteHeader(http.StatusBadRequest)
		rsp.Write([]byte("error parsing media type"))
		return
	}
	if mediatype != "multipart/form-data" {
		rsp.WriteHeader(http.StatusBadRequest)
		rsp.Write([]byte(fmt.Sprintf("bad media type [outer=%q]", mediatype)))
		return
	}

	rdr := multipart.NewReader(req.Body, params["boundary"])

	var comment []byte
	var content []byte
	var filename []byte
	ok := true

	for {
		part, err := rdr.NextPart()
		if err != nil {
			break
		}

		v := part.Header.Get("Content-Disposition")
		cdHead, cdParms, err := mime.ParseMediaType(v)
		if err != nil {
			fmt.Printf("Error parsing %q: %s\n", v, err)
			rsp.WriteHeader(http.StatusBadRequest)
			rsp.Write([]byte(fmt.Sprintf("bad media type [inner=%q]", v)))
			return
		}
		fmt.Printf("disposition %q params %v\n", cdHead, cdParms)
		fmt.Printf("Part: %v\n", part.Header)

		fn := part.FormName()
		fmt.Printf("Name: %q\n", fn)
		if fn == "comment" {
			comment, err = ioutil.ReadAll(part)
			if err != nil {
				ok = false
			}
		} else if fn == "filename" {
			filename, err = ioutil.ReadAll(part)
			if err != nil {
				ok = false
			}
		} else if fn == "body" {
			content, err = ioutil.ReadAll(part)
			if err != nil {
				ok = false
			}
		}
	}
	if ok && comment != nil && content != nil && filename != nil {
		fname := path.Clean(string(filename))
		if path.IsAbs(fname) {
			fname = fname[1:]
		}
		fmt.Printf("%d bytes of comment, %d bytes of %q\n",
			len(comment),
			len(content),
			fname)
		answer := &Answer{
			Status: "ok",
		}

		err = s.PublishFile(fname, string(comment), content)
		if err != nil {
			answer.Status = "failed"
		}
		rspbuf, _ := json.Marshal(answer)
		rsp.Write(rspbuf)
		rsp.Write([]byte("\n"))
	} else {
		rsp.WriteHeader(http.StatusBadRequest)
		rsp.Write([]byte("Error trying to process upload\n"))
	}
}

func (s *EditServer) GitCommit(fpath string, comment string) {
	repo, _ := git.OpenRepository(s.root)
	idx, _ := repo.Index()
	idx.AddByPath(fpath)
	treeId, _ := idx.WriteTree()

	sig := &git.Signature{
		Name:  "Rand Om Hacker",
		Email: "random@hacker.com",
		When:  time.Now(),
	}

	cb, _ := repo.Head()

	tree, _ := repo.LookupTree(treeId)
	if cb == nil {
		repo.CreateCommit("HEAD", sig, sig, comment, tree)
	} else {
		ct, _ := repo.LookupCommit(cb.Target())
		repo.CreateCommit("HEAD", sig, sig, comment, tree, ct)
	}
	repo.CheckoutIndex(idx, nil)
}

type Answer struct {
	Status string `json:"status"`
}

func (s *EditServer) PublishFile(fpath string, comment string, body []byte) error {
	f, err := os.Create(path.Join(s.root, fpath))
	if err != nil {
		log.Printf("Error writing file: %s", err)
		return err
	}
	f.Write(body)
	f.Close()

	s.GitCommit(fpath, comment)
	return nil
}
