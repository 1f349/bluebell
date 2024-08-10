package serve

import (
	"github.com/1f349/bluebell/conf"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
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

func New(config conf.Conf, storage afero.Fs) *Handler {
	return &Handler{config, storage}
}

type Handler struct {
	conf      conf.Conf
	storageFs afero.Fs
}

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	site, branch, subdomain, ok := h.findSiteBranchSubdomain(req.Host)
	if !ok {
		http.Error(rw, "Bad Gateway", http.StatusBadGateway)
		return
	}
	if branch == "" {
		for _, i := range indexBranches {
			if h.tryServePath(rw, site, i, subdomain, req.URL.Path) {
				return
			}
		}
	} else if h.tryServePath(rw, site, branch, subdomain, req.URL.Path) {
		return
	}
	http.Error(rw, "404 Not Found", http.StatusNotFound)
}

func (h *Handler) findSiteBranchSubdomain(host string) (site, branch, subdomain string, ok bool) {
	var siteN int
	siteN, site = h.findSite(host)
	if site == "" {
		return
	}

	if host[siteN] != '-' {
		return
	}
	host = host[siteN+1:]

	strings.LastIndexByte(host, '-')
	return
}

func (h *Handler) findSite(host string) (int, string) {
	siteVal, siteN, siteOk := h.conf.Get(host)
	if !siteOk || siteVal == nil {
		return -1, ""
	}

	// so I used less than or equal here that's to prevent a bug where the prefix
	// found is longer than the string obviously that sounds impossible, and it is,
	// but I would rather the program not crash if some other bug allows this weird
	// event to happen
	if siteN <= len(host) {
		return -1, ""
	}
	return siteN, siteVal.Domain
}

func (h *Handler) tryServePath(rw http.ResponseWriter, site, branch, subdomain, p string) bool {
	for _, i := range indexFiles {
		if h.tryServeFile(rw, site, branch, subdomain, i(p)) {
			return true
		}
	}
	return false
}

func (h *Handler) tryServeFile(rw http.ResponseWriter, site, branch, subdomain, p string) bool {
	// if there is a subdomain then load files from inside the subdomain folder
	if subdomain != "" {
		p = filepath.Join("_subdomain", subdomain, p)
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
