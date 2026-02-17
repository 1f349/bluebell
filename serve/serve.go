package serve

import (
	"context"
	_ "embed"
	"github.com/1f349/bluebell/database"
	"github.com/1f349/bluebell/logger"
	"github.com/spf13/afero"
	"html/template"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	//go:embed missing-branch.go.html
	missingBranchHtml     string
	missingBranchTemplate = template.Must(template.New("missingBranchHtml").Parse(missingBranchHtml))

	indexFiles = []func(p string) string{
		func(p string) string { return p },
		func(p string) string { return p + ".html" },
		func(p string) string { return path.Join(p, "index.html") },
	}
)

func isInvalidIndexPath(p string) bool {
	switch p {
	case ".", ".html":
		return true
	}
	return false
}

const (
	BetaCookieName       = "__bluebell-site-beta"
	BetaSwitchQuery      = "__bluebell-switch-beta"
	BetaSwitchResetQuery = "__bluebell-reset-beta"
	BetaExpiry           = 24 * time.Hour

	NoCacheQuery = "__bluebell-no-cache"
)

type serveQueries interface {
	GetLastUpdatedByDomainBranch(ctx context.Context, params database.GetLastUpdatedByDomainBranchParams) (time.Time, error)
}

func New(storage afero.Fs, db serveQueries) *Handler {
	return &Handler{storage, db}
}

type Handler struct {
	storageFs afero.Fs
	db        serveQueries
}

func cacheBuster(rw http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	q.Del(BetaSwitchQuery)
	q.Del(BetaSwitchResetQuery)
	q.Set(NoCacheQuery, strconv.FormatInt(time.Now().Unix(), 16))
	req.URL.RawQuery = q.Encode()

	header := rw.Header()
	header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
	http.Redirect(rw, req, req.URL.RequestURI(), http.StatusFound)
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet, http.MethodHead:
		break
	default:
		http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
	}

	q := req.URL.Query()

	// init cookie
	baseCookie := &http.Cookie{
		Name:     BetaCookieName,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	// reset beta
	if q.Has(BetaSwitchResetQuery) {
		baseCookie.MaxAge = -1
		http.SetCookie(rw, baseCookie)
		cacheBuster(rw, req)
		return
	}

	// detect beta switch path
	if q.Has(BetaSwitchQuery) {
		// set beta branch
		baseCookie.Value = q.Get(BetaSwitchQuery)
		baseCookie.Expires = time.Now().Add(BetaExpiry)
		http.SetCookie(rw, baseCookie)
		cacheBuster(rw, req)
		return
	}

	// read the beta cookie
	branchCookie, _ := req.Cookie(BetaCookieName)
	var branch = "@"
	if branchCookie != nil {
		branch += branchCookie.Value
	}

	updated, err := h.db.GetLastUpdatedByDomainBranch(req.Context(), database.GetLastUpdatedByDomainBranchParams{Domain: host, Branch: branch})
	if err != nil {
		rw.WriteHeader(http.StatusMisdirectedRequest)
		_ = missingBranchTemplate.Execute(rw, struct{ Host string }{host})
		logger.Logger.Debug("Branch is not available", "host", host, "branch", branch, "err", err)
		return
	}

	if h.tryServePath(rw, req, host, branch, updated, req.URL.Path) {
		return // page has been served
	}

	// tryServePath found no matching files
	http.Error(rw, "404 Not Found", http.StatusNotFound)
	logger.Logger.Debug("No matching file was found")
}

// tryServePath attempts to find a valid path from the indexFiles list
func (h *Handler) tryServePath(rw http.ResponseWriter, req *http.Request, site, branch string, updated time.Time, p string) bool {
	for _, i := range indexFiles {
		// skip invalid paths "." and ".html"
		p2 := path.Clean(i(p))
		if isInvalidIndexPath(p2) {
			continue
		}

		if h.tryServeFile(rw, req, site, branch, updated, p2) {
			return true
		}
	}
	return false
}

// tryServeFile attempts to serve the content of a file if the file can be found
//
// If a matching file can be found or an internal error has occurred then the return value is true to prevent further changes to the response.
//
// If branch == "@" then time based caching is enabled for subsequent page loads. Otherwise, time based caching is disabled to prevent stale beta content from being cached.
func (h *Handler) tryServeFile(rw http.ResponseWriter, req *http.Request, site, branch string, updated time.Time, p string) bool {
	// prevent path traversal
	if containsPathTraversal(site) || containsPathTraversal(branch) || containsPathTraversal(p) {
		http.Error(rw, "400 Bad Request", http.StatusBadRequest)
		return true
	}

	servePath := filepath.Join(site, branch, p)
	logger.Logger.Debug("Serving file", "full", servePath, "site", site, "branch", branch, "file", p)
	open, err := h.storageFs.Open(servePath)
	switch {
	case err == nil:
		defer open.Close()

		// ignore directories
		stat, err := open.Stat()
		if err != nil || stat.IsDir() {
			return false
		}

		// disable timed cache for non-main branches
		if branch != "@" {
			updated = time.Time{}
		}
		http.ServeContent(rw, req, p, updated, open)
	case os.IsNotExist(err):
		// check next path
		return false
	default:
		http.Error(rw, "500 Internal Server Error", http.StatusInternalServerError)
	}
	return true
}

func containsPathTraversal(p string) bool {
	if !strings.Contains(p, "..") {
		return false
	}
	return slices.Contains(strings.FieldsFunc(p, isSlashRune), "..")
}

func isSlashRune(r rune) bool { return r == '/' || r == '\\' }
