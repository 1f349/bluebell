package serve

import (
	"github.com/1f349/site-hosting/config"
	"github.com/spf13/afero"
	"testing"
)

func makeConfig(f afero.Fs) (*config.Config, error) {
	c := config.New(f)
	return c, c.Load()
}

func TestName(t *testing.T) {
	f := afero.NewMemMapFs()
	h := &Handler{
		storageFs: f,
		conf: config.Testable([]config.SiteConf{
			{Domain: "example.com", Token: "abcd1234"},
		}),
	}
	h.findSiteBranchSubdomain("example-com-test")
	site, branch := h.findSiteBranch("example_com_test")
}

func TestHandler_Handle(t *testing.T) {
	f := afero.NewMemMapFs()
	h := &Handler{
		storageFs: f,
		conf:      &config.Config{},
	}
	h.Handle()
}
