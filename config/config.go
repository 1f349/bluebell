package config

import (
	"fmt"
	"github.com/MrMelon54/trie"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	"sync"
)

type Config struct {
	fs afero.Fs
	l  *sync.RWMutex
	m  *trie.Trie[SiteConf]
}

type SiteConf struct {
	Domain string `json:"domain"`
	Token  string `json:"token"`
}

func New(storageFs afero.Fs) *Config {
	return &Config{
		fs: storageFs,
		l:  new(sync.RWMutex),
		m:  trie.BuildFromMap(map[string]SiteConf{}),
	}
}

func Testable(sites []SiteConf) *Config {
	c := &Config{}
	c.loadSlice(sites)
	return c
}

func (c *Config) Load() error {
	open, err := c.fs.Open("sites.yml")
	if err != nil {
		return fmt.Errorf("failed to open sites.yml: %w", err)
	}
	var a []SiteConf
	err = yaml.NewDecoder(open).Decode(&a)
	if err != nil {
		return fmt.Errorf("failed to parse yaml: %w", err)
	}

	c.loadSlice(a)
	return nil
}

func (c *Config) loadSlice(sites []SiteConf) {
	m := make(map[string]SiteConf, len(sites))

	for _, i := range sites {
		m[c.slugFromDomain(i.Domain)] = i
	}

	t := trie.BuildFromMap(m)

	c.l.Lock()
	c.m = t
	c.l.Unlock()
}

func (c *Config) slugFromDomain(domain string) string {
	a := []byte(domain)
	for i := range a {
		switch {
		case a[i] == '-':
			// skip
		case a[i] >= 'A' && a[i] <= 'Z':
			a[i] += 32
		case a[i] >= 'a' && a[i] <= 'z':
			// skip
		case a[i] >= '0' && a[i] <= '9':
			// skip
		default:
			a[i] = '-'
		}
	}
	return string(a)
}

func (c *Config) Get(key string) (*SiteConf, int, bool) {
	return c.getInternal(c.slugFromDomain(key))
}

func (c *Config) getInternal(key string) (*SiteConf, int, bool) {
	c.l.RLock()
	defer c.l.RUnlock()
	return c.m.SearchPrefixInString(key)
}
