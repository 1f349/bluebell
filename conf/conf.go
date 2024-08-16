package conf

type Conf struct {
	Listen string `yaml:"listen"`
	DB     string `yaml:"db"`
	Domain string `yaml:"domain"`
}

func SlugFromDomain(domain string) string {
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
