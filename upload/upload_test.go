package upload

import (
	"bytes"
	_ "embed"
	"github.com/1f349/bluebell/conf"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	//go:embed test-archive.tar.gz
	testArchiveTarGz []byte
	//go:embed test-sites.yml
	testSitesYml []byte
)

func assertUploadedFile(t *testing.T, fs afero.Fs) {
	// check uploaded file exists
	stat, err := fs.Stat("example.com/main/test.txt")
	assert.NoError(t, err)
	assert.False(t, stat.IsDir())
	assert.Equal(t, int64(13), stat.Size())

	// check contents
	o, err := fs.Open("example.com/main/test.txt")
	assert.NoError(t, err)
	all, err := io.ReadAll(o)
	assert.NoError(t, err)
	assert.Equal(t, "Hello world!\n", string(all))
}

func TestHandler_Handle(t *testing.T) {
	f := afero.NewMemMapFs()
	conf := conf.New(f)
	h := &Handler{f, conf}
	create, err := f.Create("sites.yml")
	assert.NoError(t, err)
	_, err = create.Write(testSitesYml)
	assert.NoError(t, err)
	assert.NoError(t, create.Close())
	assert.NoError(t, conf.Load())

	mpBuf := new(bytes.Buffer)
	mp := multipart.NewWriter(mpBuf)
	file, err := mp.CreateFormFile("upload", "test-archive.tar.gz")
	assert.NoError(t, err)
	_, err = file.Write(testArchiveTarGz)
	assert.NoError(t, err)
	assert.NoError(t, mp.Close())
	req, err := http.NewRequest(http.MethodPost, "https://example.com/u?site=example.com&branch=main", mpBuf)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer abcd1234")
	req.Header.Set("Content-Type", mp.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Handle(rec, req, httprouter.Params{})
	res := rec.Result()
	assert.Equal(t, http.StatusAccepted, res.StatusCode)
	assert.NotNil(t, res.Body)
	all, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.Equal(t, "", string(all))

	assertUploadedFile(t, f)
}

func TestHandler_extractTarGzUpload(t *testing.T) {
	fs := afero.NewMemMapFs()
	conf := conf.New(fs)
	h := &Handler{fs, conf}
	buffer := bytes.NewBuffer(testArchiveTarGz)
	assert.NoError(t, h.extractTarGzUpload(buffer, "example.com", "main"))

	assertUploadedFile(t, fs)
}
