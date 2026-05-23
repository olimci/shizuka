package build

import (
	"strings"

	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/decodeutil"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
)

func isPageSourceExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".html":
		return true
	default:
		_, ok := decodeutil.FormatExt(ext)
		return ok
	}
}

func attachPageFileMeta(page *transforms.Page, source string) {
	if page == nil {
		return
	}

	info, err := fileutil.Info(source)
	if err != nil {
		return
	}

	page.File = transforms.PageFileMeta{
		Available: true,
		Created:   info.Created,
		Updated:   info.Updated,
		Size:      info.Size,
	}
	if page.Created.IsZero() && !info.Created.IsZero() {
		page.Created = info.Created
	}
	if page.Updated.IsZero() && !info.Updated.IsZero() {
		page.Updated = info.Updated
	}
	if !page.Updated.IsZero() {
		page.PubDate = page.Updated
	} else if !page.Created.IsZero() {
		page.PubDate = page.Created
	}
}
