package conf

import "github.com/mrmelon54/trie"

type Conf struct {
	Listen string `yaml:"listen"`
	//fs     afero.Fs
	//l      *sync.RWMutex
	m *trie.Trie[SiteConf]
}

type SiteConf struct {
	Domain string `json:"domain"`
	Token  string `json:"token"`
}

func (c *Conf) slugFromDomain(domain string) string {
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
