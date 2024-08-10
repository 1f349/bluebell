package conf

import (
	_ "embed"
	"github.com/mrmelon54/trie"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

//go:embed test-sites.yml
var testSitesYml []byte

func TestConfig_Load(t *testing.T) {
	f := afero.NewMemMapFs()
	create, err := f.Create("sites.yml")
	assert.NoError(t, err)
	_, err = create.Write(testSitesYml)
	assert.NoError(t, err)
	assert.NoError(t, create.Close())

	c := New(f)
	assert.NoError(t, c.Load())
	val, ok := c.m.GetByString("example-com")
	assert.True(t, ok)
	assert.Equal(t, SiteConf{Domain: "example.com", Token: "abcd1234"}, *val)
}

func TestConfig_loadSlice(t *testing.T) {
	c := &Conf{l: new(sync.RWMutex)}
	c.loadSlice([]SiteConf{
		{Domain: "example.com", Token: "abcd1234"},
	})
	a, ok := c.m.GetByString("example-com")
	assert.True(t, ok)
	assert.Equal(t, SiteConf{Domain: "example.com", Token: "abcd1234"}, *a)
}

func TestConfig_slugFromDomain(t *testing.T) {
	c := &Conf{}
	assert.Equal(t, "---------------", c.slugFromDomain("!\"#$%&'()*+,-./"))
	assert.Equal(t, "0123456789", c.slugFromDomain("0123456789"))
	assert.Equal(t, "-------", c.slugFromDomain(":;<=>?@"))
	assert.Equal(t, "abcdefghijklmnopqrstuvwxyz", c.slugFromDomain("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
	assert.Equal(t, "------", c.slugFromDomain("[\\]^_`"))
	assert.Equal(t, "abcdefghijklmnopqrstuvwxyz", c.slugFromDomain("abcdefghijklmnopqrstuvwxyz"))
	assert.Equal(t, "----", c.slugFromDomain("{|}~"))
}

func FuzzConfig_slugFromDomain(f *testing.F) {
	c := &Conf{}
	f.Fuzz(func(t *testing.T, a string) {
		b := c.slugFromDomain(a)
		if len(a) != len(b) {
			t.Fatalf("value '%s' (%d) did not match lengths with the output '%s' (%d)", a, len(a), b, len(b))
		}
	})
}

func TestConfig_Get(t *testing.T) {
	c := &Conf{l: new(sync.RWMutex), m: &trie.Trie[SiteConf]{}}
	c.loadSlice([]SiteConf{
		{Domain: "example.com", Token: "abcd1234"},
	})
	val, n, ok := c.Get("example.com")
	assert.True(t, ok)
	assert.Equal(t, 11, n)
	assert.Equal(t, SiteConf{Domain: "example.com", Token: "abcd1234"}, *val)
}
