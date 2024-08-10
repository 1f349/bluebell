package upload

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/1f349/bluebell/conf"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

func New(storage afero.Fs) *Handler {
	return &Handler{storage, conf}
}

type Handler struct {
	storageFs afero.Fs
	conf      *conf.Conf
}

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	q := req.URL.Query()
	site := q.Get("site")
	branch := q.Get("branch")

	siteConf, siteN, siteOk := h.conf.Get(site)
	if !siteOk || siteN != len(site) || siteConf == nil {
		http.Error(rw, "400 Bad Request", http.StatusBadRequest)
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
	if fileHeader.Size > 1074000000 {
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
