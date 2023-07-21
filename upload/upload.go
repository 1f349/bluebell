package upload

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func New(storagePath string) *Handler {
	fs := afero.NewBasePathFs(afero.NewOsFs(), storagePath)
	return &Handler{fs}
}

type Handler struct {
	storageFs afero.Fs
}

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	site := params.ByName("site")
	branch := req.URL.Query().Get("branch")

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
	storeFs := afero.NewBasePathFs(h.storageFs, filepath.Join(site, branch))

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
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}

		err = storeFs.MkdirAll(filepath.Dir(next.Name), os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to make directory tree: %w", err)
		}

		create, err := storeFs.Create(next.Name)
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
