package conf

type Conf struct {
	Listen ListenConf `yaml:"listen"`
	DB     string     `yaml:"db"`
}

type ListenConf struct {
	Http string `yaml:"http"`
	Api  string `yaml:"api"`
}
