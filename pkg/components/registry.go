package components

var (
	components = map[string]ComponentInstaller{}
)

func RegisterComponent(comp ComponentInstaller) {
	components[comp.Name()] = comp
}

func GetComponentByName(name string) ComponentInstaller {
	return components[name]
}
