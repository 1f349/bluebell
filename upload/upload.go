package upload

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/1f349/bluebell/database"
	"github.com/1f349/bluebell/hook"
	"github.com/1f349/bluebell/logger"
	"github.com/1f349/bluebell/peers"
	"github.com/1f349/bluebell/validation"
	"github.com/1f349/syncmap"
	"github.com/dustin/go-humanize"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var indexBranches = []string{
	"main",
	"master",
}

type uploadQueries interface {
	GetSiteByDomain(ctx context.Context, domain string) (database.Site, error)
	AddBranch(ctx context.Context, arg database.AddBranchParams) error
	UpdateBranch(ctx context.Context, arg database.UpdateBranchParams) error
}

func New(storage afero.Fs, db uploadQueries, hook *hook.Hook, peerManager *peers.Peers) *Handler {
	return &Handler{storageFs: storage, db: db, postHook: hook}
}

const maxFileSize = 1 * humanize.GiByte

type Handler struct {
	storageFs afero.Fs
	db        uploadQueries
	mu        syncmap.Map[string, *sync.Mutex]
	postHook  *hook.Hook
}

func (h *Handler) Handle(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	site := params.ByName("site")
	branch := strings.TrimPrefix(params.ByName("branch"), "/")

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
		if errors.Unwrap(err) == nil {
			http.Error(rw, fmt.Sprintf("Invalid upload: %s", err), http.StatusBadRequest)
		} else {
			logger.Logger.Error("Invalid upload", "site", site, "branch", branch, "err", err)
			http.Error(rw, "Invalid upload: [Internal Server Error]", http.StatusBadRequest)
		}
		return
	}

	rw.WriteHeader(http.StatusAccepted)
}

func (h *Handler) extractTarGzUpload(fileData io.Reader, site, branch string) error {
	if !validation.IsValidSite(site) {
		return fmt.Errorf("invalid site name: %s", site)
	}
	if !validation.IsValidBranch(branch) {
		return fmt.Errorf("invalid branch name: %s", branch)
	}
	if slices.Contains(indexBranches, branch) {
		branch = ""
	}

	_, err := h.db.GetSiteByDomain(context.Background(), site)
	if err != nil {
		return fmt.Errorf("invalid site: %w", err)
	}

	key := site + "@" + branch

	// ensure upload mutex is locked
	actual, _ := h.mu.LoadOrStore(key, new(sync.Mutex))
	actual.Lock()
	defer func() {
		// The mutex is no longer used so delete it here to safe memory in a "lots of
		// sites" configuration. Delete should happen first to prevent another upload
		// reusing the mutex.
		h.mu.Delete(key)
		actual.Unlock()
	}()

	siteBranchPath := filepath.Join(site, "@"+branch)
	siteBranchOldPath := filepath.Join(site, "old@"+branch)
	siteBranchWorkPath := filepath.Join(site, "work@"+branch)

	// try the new "old@[...]" and old "@[...].old" paths
	err = h.storageFs.RemoveAll(siteBranchPath + ".old")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove old site branch %s: %w", siteBranchPath, err)
	}
	err = h.storageFs.RemoveAll(siteBranchOldPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove old site branch %s: %w", siteBranchPath, err)
	}

	err = h.storageFs.MkdirAll(siteBranchWorkPath, fs.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to make site directory: %w", err)
	}
	branchFs := afero.NewBasePathFs(h.storageFs, siteBranchWorkPath)

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

		if next.FileInfo().IsDir() {
			continue
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

	// call the post hook script
	err = h.postHook.Run(site, branch)
	if err != nil {
		return err
	}

	// TODO(melon): I would love to use unix.Renameat2 but due to afero this will not work

	err = h.storageFs.Rename(siteBranchPath, siteBranchOldPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to save an old copy of the site: %w", err)
	}

	err = h.storageFs.Rename(siteBranchWorkPath, siteBranchPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to save an old copy of the site: %w", err)
	}

	n := time.Now().UTC()

	err = h.db.AddBranch(context.Background(), database.AddBranchParams{
		Branch:     "@" + branch,
		Domain:     site,
		LastUpdate: n,
		Enable:     true,
	})
	if err != nil {
		return h.db.UpdateBranch(context.Background(), database.UpdateBranchParams{
			Branch:     "@" + branch,
			Domain:     site,
			LastUpdate: n,
		})
	}
	return nil
}
