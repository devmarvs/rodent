package modules

import "fyne.io/fyne/v2"

type Module interface {
	Name() string
	Content() fyne.CanvasObject
}

func Registered() []Module {
	return []Module{
		&scannerModule{},
		&networkMapperModule{},
		&vulnerabilityModule{},
	}
}
