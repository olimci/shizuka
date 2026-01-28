package keys

import (
	"fmt"
	"html/template"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
)

type K[T any] string

const (
	Config    = K[*config.Config]("config")
	Options   = K[*config.Options]("options")
	Pages     = K[*transforms.PageTree]("pages")
	Site      = K[*transforms.Site]("site")
	Templates = K[*template.Template]("templates")
	BuildMeta = K[*transforms.BuildMeta]("buildmeta")
)

// GetAs retrieves a value from the registry as the specified type.
func GetAs[T any](m manifest.RegistryGetter, k K[T]) T {
	v, ok := m.Get(string(k))
	if !ok {
		panic(fmt.Sprintf("manifest: missing registry key %q", string(k)))
	}
	vt, ok := v.(T)
	if !ok {
		panic(fmt.Sprintf("manifest: registry key %q has type %T", string(k), v))
	}
	return vt
}

func SetAs[T any](m manifest.RegistrySetter, k K[T], v T) {
	m.Set(string(k), v)
}
