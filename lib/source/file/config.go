package file

type Config struct {
	Path string `yaml:"path"`
	// Watch makes a change of the file trigger an update at once. On by
	// default.
	Watch *bool `yaml:"watch"`
}

func (c Config) watch() bool {
	return c.Watch == nil || *c.Watch
}
