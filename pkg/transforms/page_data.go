package transforms

import (
	"encoding/json"
	"io/fs"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type DataPage struct {
	Frontmatter
	Body string `toml:"body" yaml:"body" json:"body"`
}

func (d *DataPage) Front() *Frontmatter {
	return &d.Frontmatter
}

func buildTOML(fsys fs.FS, path string) (*Frontmatter, []byte, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	data := new(DataPage)

	if _, err := toml.NewDecoder(file).Decode(data); err != nil {
		return nil, nil, err
	}

	return data.Front(), []byte(data.Body), nil
}

func buildYaml(fsys fs.FS, path string) (*Frontmatter, []byte, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	data := new(DataPage)

	if err := yaml.NewDecoder(file).Decode(data); err != nil {
		return nil, nil, err
	}

	return data.Front(), []byte(data.Body), nil
}

func buildJSON(fsys fs.FS, path string) (*Frontmatter, []byte, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	data := new(DataPage)

	if err := json.NewDecoder(file).Decode(data); err != nil {
		return nil, nil, err
	}

	return data.Front(), []byte(data.Body), nil
}
