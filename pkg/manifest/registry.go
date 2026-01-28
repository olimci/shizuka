package manifest

type RegistryGetter interface {
	Get(string) (any, bool)
}

type RegistrySetter interface {
	Set(string, any)
}
