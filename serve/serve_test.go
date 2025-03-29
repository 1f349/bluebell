package serve

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/1f349/bluebell/database"
	"github.com/1f349/bluebell/logger"
	"github.com/charmbracelet/log"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func init() {
	logger.Logger.SetLevel(log.DebugLevel)
}

type fakeServeDB struct {
	branch string
}

//goland:noinspection HttpUrlsUsage
const httpPrefix = "http://"

func (f *fakeServeDB) GetLastUpdatedByDomainBranch(_ context.Context, params database.GetLastUpdatedByDomainBranchParams) (time.Time, error) {
	if params.Domain == "example.com" && params.Branch == "@"+f.branch {
		return time.Now(), nil
	}
	return time.Time{}, sql.ErrNoRows
}

func TestHandler_ServeHTTP(t *testing.T) {
	for _, branch := range []string{"", "test", "dev"} {
		t.Run(branch+" branch", func(t *testing.T) {
			serveTest(t, "example.com", branch, "example.com/@"+branch+"/index.html")
			serveTest(t, "example.com/hello-world", branch, "example.com/@"+branch+"/hello-world.html")
			serveTest(t, "example.com/hello-world", branch, "example.com/@"+branch+"/hello-world/index.html")
			serveTest(t, "example.com/hello-world", branch, "example.com/@"+branch+"/hello-world")
		})

		t.Run("switch to "+branch+" branch", func(t *testing.T) {
			h := New(afero.NewMemMapFs(), &fakeServeDB{})

			req := httptest.NewRequest(http.MethodGet, httpPrefix+"example.com/?__bluebell-switch-beta="+branch, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			res := rec.Result()
			assert.Equal(t, http.StatusFound, rec.Code)
			assert.NotNil(t, res.Body)
			all, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			assert.Contains(t, string(all), "<a href=\"/?__bluebell-no-cache=")
			assert.Contains(t, string(all), "\">Found</a>.\n\n")

			cookies := res.Cookies()
			assert.Len(t, cookies, 1)
			assert.Equal(t, cookies[0].Name, "__bluebell-site-beta")
			assert.Equal(t, cookies[0].Value, branch)
			assert.Equal(t, cookies[0].Path, "/")
			assert.Equal(t, cookies[0].HttpOnly, true)
			assert.Equal(t, cookies[0].SameSite, http.SameSiteLaxMode)
		})
	}
}

func serveTest(t *testing.T, address string, branch string, name string) {
	t.Run(fmt.Sprintf("serveTest \"%s\" (%s) -> \"%s\"", address, branch, name), func(t *testing.T) {
		fs := afero.NewMemMapFs()
		assert.NoError(t, fs.MkdirAll(filepath.Dir(name), os.ModePerm))
		assert.NoError(t, afero.WriteFile(fs, name, []byte("Hello World\n"), 0666))
		h := New(fs, &fakeServeDB{branch: branch})

		req := httptest.NewRequest(http.MethodGet, httpPrefix+address, nil)
		if branch != "" {
			req.AddCookie(&http.Cookie{
				Name:  "__bluebell-site-beta",
				Value: branch,
			})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		res := rec.Result()
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.NotNil(t, res.Body)
		all, err := io.ReadAll(res.Body)
		assert.NoError(t, err)
		assert.Equal(t, "Hello World\n", string(all))
	})
}
