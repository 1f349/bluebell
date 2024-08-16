package serve

import (
	"context"
	"github.com/1f349/bluebell/conf"
	"github.com/1f349/bluebell/database"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	indexBranches = []string{
		"main",
		"master",
	}
	indexFiles = []func(p string) string{
		func(p string) string { return path.Join(p, "index.html") },
		func(p string) string { return p + ".html" },
		func(p string) string { return p },
	}
)

type sitesQueries interface {
	GetSiteByDomain(ctx context.Context, domain string) (database.Site, error)
}

func New(storage afero.Fs, db sitesQueries, domain string) *Handler {
	return &Handler{storage, db, domain}
}

type Handler struct {
	storageFs afero.Fs
	db        sitesQueries
	domain    string
}

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		http.Error(rw, "Bad Gateway", http.StatusBadGateway)
		return
	}
	site, ok := strings.CutSuffix(host, "."+h.domain)
	if !ok {
		http.Error(rw, "Bad Gateway", http.StatusBadGateway)
		return
	}
	site = conf.SlugFromDomain(site)
	branch := req.URL.User.Username()
	if branch == "" {
		for _, i := range indexBranches {
			if h.tryServePath(rw, site, i, req.URL.Path) {
				return
			}
		}
	} else if h.tryServePath(rw, site, branch, req.URL.Path) {
		return
	}
	http.Error(rw, "404 Not Found", http.StatusNotFound)
}

func (h *Handler) tryServePath(rw http.ResponseWriter, site, branch, p string) bool {
	for _, i := range indexFiles {
		if h.tryServeFile(rw, site, branch, i(p)) {
			return true
		}
	}
	return false
}

func (h *Handler) tryServeFile(rw http.ResponseWriter, site, branch, p string) bool {
	// prevent path traversal
	if strings.Contains(site, "..") || strings.Contains(branch, "..") || strings.Contains(p, "..") {
		http.Error(rw, "400 Bad Request", http.StatusBadRequest)
		return true
	}
	open, err := h.storageFs.Open(filepath.Join(site, branch, p))
	switch {
	case err == nil:
		rw.WriteHeader(http.StatusOK)
		_, _ = io.Copy(rw, open)
	case os.IsNotExist(err):
		// check next path
		return false
	default:
		http.Error(rw, "500 Internal Server Error", http.StatusInternalServerError)
	}
	return true
}
