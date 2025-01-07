package upload

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/1f349/bluebell/database"
	"github.com/dustin/go-humanize"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
)

var indexBranches = []string{
	"main",
	"master",
}

func containsOnly(s string, f func(r rune) bool) bool {
	for _, r := range []rune(s) {
		if !f(r) {
			return false
		}
	}
	return true
}

func isValidSite(site string) bool {
	if len(site) < 1 || site[0] == '-' {
		return false
	}
	switch site[0] {
	case '-':
		return false
	}
	return containsOnly(site, func(r rune) bool {
		return isAlphanumericOrDash(r) || r == '.'
	})
}

func isValidBranch(branch string) bool {
	if len(branch) < 1 {
		return false
	}
	switch branch[0] {
	case '-', '/':
		return false
	}
	if branch[len(branch)-1] == '/' {
		return false
	}
	return containsOnly(branch, func(r rune) bool {
		return isAlphanumericOrDash(r) || r == '/' || r == '.'
	})
}

func isAlphanumericOrDash(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r == '-', r == '_':
		return true
	default:
		return false
	}
}

type sitesQueries interface {
	GetSiteByDomain(ctx context.Context, domain string) (database.Site, error)
}

func New(storage afero.Fs, db sitesQueries) *Handler {
	return &Handler{storage, db}
}

const maxFileSize = 1 * humanize.GiByte

type Handler struct {
	storageFs afero.Fs
	db        sitesQueries
}

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	site := params.ByName("site")
	branch := params.ByName("branch")

	siteConf, err := h.db.GetSiteByDomain(req.Context(), site)
	if err != nil {
		http.Error(rw, "", http.StatusNotFound)
		return
	}
	token, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(siteConf.Token)) == 0 {
		http.Error(rw, "403 Forbidden", http.StatusForbidden)
		return
	}

	fileData, fileHeader, err := req.FormFile("upload")
	if err != nil {
		http.Error(rw, "Missing file upload", http.StatusBadRequest)
		return
	}

	// if file is bigger than maxFileSize
	if fileHeader.Size > maxFileSize {
		http.Error(rw, "File too big", http.StatusInsufficientStorage)
		return
	}

	err = h.extractTarGzUpload(fileData, site, branch)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Invalid upload: %s", err), http.StatusBadRequest)
		return
	}

	rw.WriteHeader(http.StatusAccepted)
}

func (h *Handler) extractTarGzUpload(fileData io.Reader, site, branch string) error {
	if !isValidSite(site) {
		return fmt.Errorf("invalid site name: %s", site)
	}
	if !isValidBranch(branch) {
		return fmt.Errorf("invalid branch name: %s", branch)
	}
	if slices.Contains(indexBranches, branch) {
		branch = ""
	}

	_, err := h.db.GetSiteByDomain(context.Background(), site)
	if err != nil {
		return fmt.Errorf("invalid site: %w", err)
	}

	siteBranchPath := filepath.Join(site, "@"+branch)
	siteBranchOldPath := filepath.Join(site, "old@"+branch)

	// try the new "old@[...]" and old "@[...].old" paths
	err = h.storageFs.RemoveAll(siteBranchPath + ".old")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove old site branch %s: %w", siteBranchPath, err)
	}
	err = h.storageFs.RemoveAll(siteBranchOldPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove old site branch %s: %w", siteBranchPath, err)
	}

	err = h.storageFs.Rename(siteBranchPath, siteBranchOldPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to save an old copy of the site: %w", err)
	}

	err = h.storageFs.MkdirAll(siteBranchPath, fs.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to make site directory: %w", err)
	}
	branchFs := afero.NewBasePathFs(h.storageFs, siteBranchPath)

	// decompress gzip wrapper
	gzipReader, err := gzip.NewReader(fileData)
	if err != nil {
		return fmt.Errorf("invalid gzip file: %w", err)
	}

	// parse tar encoding
	tarReader := tar.NewReader(gzipReader)
	for {
		next, err := tarReader.Next()
		if err == io.EOF {
			// finished reading tar, exit now
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}

		err = branchFs.MkdirAll(filepath.Dir(next.Name), fs.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to make directory tree: %w", err)
		}

		create, err := branchFs.Create(next.Name)
		if err != nil {
			return fmt.Errorf("failed to create output file: '%s': %w", next.Name, err)
		}

		_, err = io.Copy(create, tarReader)
		if err != nil {
			return fmt.Errorf("failed to copy from archive to output file: '%s': %w", next.Name, err)
		}
	}
	return nil
}
