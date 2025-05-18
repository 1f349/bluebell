package upload

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/1f349/bluebell"
	"github.com/1f349/bluebell/database"
	"github.com/1f349/bluebell/hook"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

var (
	//go:embed test-archive.tar.gz
	testArchiveTarGz []byte
)

func initMemoryDB(t *testing.T) (*sql.DB, *database.Queries) {
	rawDB, err := sql.Open("sqlite3", "file:"+uuid.NewString()+"?mode=memory&cache=shared")
	assert.NoError(t, err)

	db, err := bluebell.InitRawDB(rawDB)
	assert.NoError(t, err)

	assert.NoError(t, db.AddSite(context.Background(), database.AddSiteParams{
		Domain: "example.com",
		Token:  "abcd1234",
	}))

	return rawDB, db
}

func assertUploadedFile(t *testing.T, fs afero.Fs, branch string) {
	switch branch {
	case "main", "master":
		branch = ""
	}

	// check uploaded file exists
	stat, err := fs.Stat("example.com/@" + branch + "/test.txt")
	assert.NoError(t, err)
	assert.False(t, stat.IsDir())
	assert.Equal(t, int64(13), stat.Size())

	stat, err = fs.Stat("example.com/work@" + branch + "/test.txt")
	assert.Error(t, err)

	// check contents
	o, err := fs.Open("example.com/@" + branch + "/test.txt")
	assert.NoError(t, err)
	all, err := io.ReadAll(o)
	assert.NoError(t, err)
	assert.Equal(t, "Hello world!\n", string(all))
}

type fakeUploadDB struct {
	branchesMu  sync.Mutex
	branchesMap map[string]database.Branch
}

func (f *fakeUploadDB) GetSiteByDomain(_ context.Context, domain string) (database.Site, error) {
	if domain == "example.com" {
		return database.Site{
			Domain: "example.com",
			Token:  "abcd1234",
		}, nil
	}
	return database.Site{}, sql.ErrNoRows
}

func (f *fakeUploadDB) AddBranch(_ context.Context, arg database.AddBranchParams) error {
	f.branchesMu.Lock()
	defer f.branchesMu.Unlock()
	if f.branchesMap == nil {
		f.branchesMap = make(map[string]database.Branch)
	}

	key := arg.Domain + "@" + arg.Branch
	_, exists := f.branchesMap[key]
	if exists {
		return fmt.Errorf("primary key constraint failed")
	}
	f.branchesMap[key] = database.Branch{
		Domain:     arg.Domain,
		Branch:     arg.Branch,
		LastUpdate: arg.LastUpdate,
		Enable:     arg.Enable,
	}
	return nil
}

func (f *fakeUploadDB) UpdateBranch(ctx context.Context, arg database.UpdateBranchParams) error {
	f.branchesMu.Lock()
	defer f.branchesMu.Unlock()
	if f.branchesMap == nil {
		f.branchesMap = make(map[string]database.Branch)
	}

	item, exists := f.branchesMap[arg.Domain+"@"+arg.Branch]
	if !exists {
		return sql.ErrNoRows
	}
	item.LastUpdate = arg.LastUpdate
	return nil
}

func TestHandler_Handle(t *testing.T) {
	fs := afero.NewMemMapFs()
	uploadFs := afero.NewMemMapFs()
	h := New(fs, uploadFs, new(fakeUploadDB), hook.New("", ""), nil)

	r := httprouter.New()
	r.POST("/u/:site/*branch", h.Handle)
	r.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("Not Found")
	})

	for _, branch := range []string{"main", "test", "dev", "fix/with-slash"} {
		t.Run(branch+" branch", func(t *testing.T) {
			mpBuf := new(bytes.Buffer)
			mp := multipart.NewWriter(mpBuf)
			file, err := mp.CreateFormFile("upload", "test-archive.tar.gz")
			assert.NoError(t, err)
			_, err = file.Write(testArchiveTarGz)
			assert.NoError(t, err)
			assert.NoError(t, mp.Close())

			req := httptest.NewRequest(http.MethodPost, "https://example.com/u/example.com/"+branch, mpBuf)
			req.Header.Set("Authorization", "Bearer abcd1234")
			req.Header.Set("Content-Type", mp.FormDataContentType())
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			res := rec.Result()
			assert.Equal(t, http.StatusAccepted, res.StatusCode)
			assert.NotNil(t, res.Body)
			all, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			assert.Equal(t, "", string(all))

			fmt.Println(fs)

			assertUploadedFile(t, fs, branch)
		})
	}
}

func TestHandler_extractTarGzUpload_fakeDB(t *testing.T) {
	extractTarGzUploadTest(t, new(fakeUploadDB))
}

func TestHandler_extractTarGzUpload_memoryDB(t *testing.T) {
	_, db := initMemoryDB(t)
	extractTarGzUploadTest(t, db)
}

func extractTarGzUploadTest(t *testing.T, db uploadQueries) {
	for _, branch := range []string{"main", "test", "dev"} {
		t.Run(branch+" branch", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			uploadFs := afero.NewMemMapFs()
			h := New(fs, uploadFs, db, hook.New("", ""), nil)
			buffer := bytes.NewBuffer(testArchiveTarGz)
			assert.NoError(t, h.extractTarGzUpload(buffer, int64(buffer.Len()), "example.com", branch))

			assertUploadedFile(t, fs, branch)
		})
	}
}

func TestHandler_extractTarGzUpload_fakeDB_multiple(t *testing.T) {
	extractTarGzUploadMultipleTest(t, new(fakeUploadDB))
}

func TestHandler_extractTarGzUpload_memoryDB_multiple(t *testing.T) {
	_, db := initMemoryDB(t)
	extractTarGzUploadMultipleTest(t, db)
}

func extractTarGzUploadMultipleTest(t *testing.T, db uploadQueries) {
	fs := afero.NewMemMapFs()
	uploadFs := afero.NewMemMapFs()
	h := New(fs, uploadFs, db, hook.New("", ""), nil)
	sig := new(atomic.Bool)
	wg := new(sync.WaitGroup)

	const callCount = 2
	wg.Add(callCount)

	for range callCount {
		go func() {
			defer wg.Done()
			buffer := newSingleBufferReader(testArchiveTarGz, sig)
			assert.NoError(t, h.extractTarGzUpload(buffer, int64(len(testArchiveTarGz)), "example.com", "main"))
			assertUploadedFile(t, fs, "main")
		}()
	}

	wg.Wait()

	assertUploadedFile(t, fs, "main")
}

type singleBufferReader struct {
	r      io.Reader
	signal *atomic.Bool
	active bool
	size   int
}

func (s *singleBufferReader) Read(b []byte) (n int, err error) {
	// only check the signal if this reader is not active
	if !s.active {
		old := s.signal.Swap(true)
		if old {
			panic("singleBufferReader: peer buffer is already being used")
		}
		s.active = true
	}
	n, err = s.r.Read(b)
	s.size -= n
	if s.size <= 0 {
		s.active = false
		s.signal.Store(false)
	}
	return n, err
}

func newSingleBufferReader(buf []byte, b *atomic.Bool) *singleBufferReader {
	return &singleBufferReader{r: bytes.NewReader(buf), signal: b, size: len(buf)}
}
