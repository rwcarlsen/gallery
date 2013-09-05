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
	Default *Config
	DefaultConfigPath = filepath.Join(os.Getenv("HOME"), ".piclibrc")
)

var DefaultSpec = &backend.Spec{
	Btype: backend.Local,
	Bparams: backend.Params{
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

func (c *Config) MakeBackend() (backend.Interface, error) {
	if specpath := os.Getenv("BACKEND_SPEC"); specpath != "" {
		f, err := os.Open(specpath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return backend.LoadSpec(f)
	} else if c.BackendSpecPath != "" {
		f, err := os.Open(c.BackendSpecPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return backend.LoadSpec(f)
	} else {
		return DefaultSpec.Make()
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
	if c.WebpicsPath == "" {
		panic("conf: cannot find webpics assets")
	}
	return c.WebpicsPath
}

