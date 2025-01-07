package upload

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/1f349/bluebell/database"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsValidSite(t *testing.T) {
	for _, i := range []struct {
		s     string
		valid bool
	}{
		{"", false},
		{"a", true},
		{"abc", true},
		{"0", true},
		{"0123456789", true},
		{"_", true},
		{"_ab", true},
		{"-", false},
		{"-ab", false},
		{".", true},
		{".ab", true},
		{"a-b", true},
		{"a_b", true},
		{"a/b", false},
		{"a.b", true},
		{"ab-", true},
		{"ab_", true},
		{"ab.", true},
		{"/ab", false},
		{"ab/", false},
	} {
		assert.Equal(t, i.valid, isValidSite(i.s), "Test failed \"%s\" - %v", i.s, i.valid)
	}
}

func TestIsValidBranch(t *testing.T) {
	for _, i := range []struct {
		s     string
		valid bool
	}{
		{"", false},
		{"a", true},
		{"abc", true},
		{"0", true},
		{"0123456789", true},
		{"_", true},
		{"_ab", true},
		{"-", false},
		{"-ab", false},
		{".", true},
		{".ab", true},
		{"a-b", true},
		{"a_b", true},
		{"a/b", true},
		{"a.b", true},
		{"ab-", true},
		{"ab_", true},
		{"ab.", true},
		{"/ab", false},
		{"ab/", false},
	} {
		assert.Equal(t, i.valid, isValidBranch(i.s), "Test failed \"%s\" - %v", i.s, i.valid)
	}
}

var (
	//go:embed test-archive.tar.gz
	testArchiveTarGz []byte
)

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

	// check contents
	o, err := fs.Open("example.com/@" + branch + "/test.txt")
	assert.NoError(t, err)
	all, err := io.ReadAll(o)
	assert.NoError(t, err)
	assert.Equal(t, "Hello world!\n", string(all))
}

type fakeUploadDB struct {
}

func (f *fakeUploadDB) GetSiteByDomain(_ context.Context, domain string) (database.Site, error) {
	if domain == "example.com" {
		return database.Site{
			ID:     1,
			Domain: "example.com",
			Token:  "abcd1234",
		}, nil
	}
	return database.Site{}, sql.ErrNoRows
}

func TestHandler_Handle(t *testing.T) {
	fs := afero.NewMemMapFs()
	h := New(fs, new(fakeUploadDB))

	r := httprouter.New()
	r.POST("/u/:site/:branch", h.Handle)
	r.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("Not Found")
	})

	for _, branch := range []string{"main", "test", "dev"} {
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

func TestHandler_extractTarGzUpload(t *testing.T) {
	for _, branch := range []string{"main", "test", "dev"} {
		t.Run(branch+" branch", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			h := New(fs, new(fakeUploadDB))
			buffer := bytes.NewBuffer(testArchiveTarGz)
			assert.NoError(t, h.extractTarGzUpload(buffer, "example.com", branch))

			assertUploadedFile(t, fs, branch)
		})
	}
}
