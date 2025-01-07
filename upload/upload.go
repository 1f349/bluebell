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

	// if file is bigger than 1GiB
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
	if slices.Contains(indexBranches, branch) {
		branch = ""
	}
	siteBranchPath := filepath.Join(site, "@"+branch)

	// try the new "old@[...]" and old "@[...].old" paths
	err := h.storageFs.RemoveAll(siteBranchPath + ".old")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove old site branch %s: %w", siteBranchPath, err)
	}
	err = h.storageFs.RemoveAll("old" + siteBranchPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove old site branch %s: %w", siteBranchPath, err)
	}

	err = h.storageFs.Rename(siteBranchPath, "old"+siteBranchPath)
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
