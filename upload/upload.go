package upload

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/1f349/bluebell/database"
	"github.com/dustin/go-humanize"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type sitesQueries interface {
	GetSiteBySlug(ctx context.Context, slug string) (database.Site, error)
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

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	q := req.URL.Query()
	site := q.Get("site")
	branch := q.Get("branch")

	site = strings.ReplaceAll(site, "*", "")

	siteConf, err := h.db.GetSiteByDomain(req.Context(), "*"+site)
	if err != nil {
		http.Error(rw, "", http.StatusNotFound)
		return
	}
	if "Bearer "+siteConf.Token != req.Header.Get("Authorization") {
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
		http.Error(rw, "File too big", http.StatusBadRequest)
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
	siteBranchPath := filepath.Join(site, branch)
	err := h.storageFs.Rename(siteBranchPath, siteBranchPath+".old")
	if err != nil && !os.IsNotExist(err) {
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
