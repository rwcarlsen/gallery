package conf

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/gallery/backend"
)

var (
	Default           *Config
	DefaultConfigPath = filepath.Join(os.Getenv("HOME"), ".piclibrc")
)

var DefaultSpec = &backend.Spec{
	Type: backend.Local,
	Params: backend.Params{
		"Root": os.Getenv("HOME"),
	},
}

const DefaultLibName = "piclib"

func init() {
	var err error
	Default, err = LoadFile(DefaultConfigPath)
	if err != nil {
		Default = &Config{}
	}
}

type Config struct {
	BackendSpecPath string
	LibraryName     string
	LogPath         string
	WebpicsPath     string
}

func Load(r io.Reader) (*Config, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

func (c *Config) Save(w io.Writer) error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

func (c *Config) SaveFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.Save(f)
}

func (c *Config) SpecPath() string {
	if specpath := os.Getenv("BACKEND_SPEC"); specpath != "" {
		return specpath
	} else {
		return c.BackendSpecPath
	}
}

func (c *Config) Backend() (backend.Interface, error) {
	s, err := c.Spec()
	if err != nil {
		return nil, err
	}
	return s.Make()
}

func (c *Config) Spec() (*backend.Spec, error) {
	if specpath := c.SpecPath(); specpath != "" {
		f, err := os.Open(specpath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return backend.LoadSpec(f)
	} else {
		return DefaultSpec, nil
	}
}

func (c *Config) LibName() string {
	if name := os.Getenv("PICLIB_NAME"); name != "" {
		return name
	} else if c.LibraryName != "" {
		return c.LibraryName
	} else {
		return DefaultLibName
	}
}

func (c *Config) LogFile() string {
	if p := os.Getenv("PICLIB_LOG"); p != "" {
		return p
	} else {
		return filepath.Join(os.Getenv("HOME"), ".picliblog")
	}
}

func (c *Config) WebpicsAssets() string {
	if p := os.Getenv("WEBPICS"); p != "" {
		return p
	} else if c.WebpicsPath != "" {
		return c.WebpicsPath
	} else {
		panic("conf: cannot find webpics assets")
	}
}
