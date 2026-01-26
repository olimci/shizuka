package transforms

import (
	"io/fs"
)

func buildMD(fsys fs.FS, path string) (*Frontmatter, []byte, error) {
	doc, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, nil, err
	}

	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		return nil, nil, err
	}

	return fm, body, nil
}
