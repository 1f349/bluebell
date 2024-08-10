package serve

import (
	"github.com/1f349/bluebell/conf"
	"github.com/spf13/afero"
	"testing"
)

func makeConfig(f afero.Fs) (*conf.Conf, error) {
	c := conf.New(f)
	return c, c.Load()
}

func TestName(t *testing.T) {
	f := afero.NewMemMapFs()
	h := &Handler{
		storageFs: f,
		conf: conf.Testable([]conf.SiteConf{
			{Domain: "example.com", Token: "abcd1234"},
		}),
	}
	h.findSiteBranchSubdomain("example-com-test")
	site, branch := h.findSiteBranch("example-com_test")
}

func TestHandler_Handle(t *testing.T) {
	f := afero.NewMemMapFs()
	h := &Handler{
		storageFs: f,
		conf:      &conf.Conf{},
	}
	h.Handle()
}
