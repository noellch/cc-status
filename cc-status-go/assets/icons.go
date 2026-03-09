// Package assets provides embedded icon resources.
package assets

import _ "embed"

var (
	//go:embed idle.png
	IconIdle []byte
	//go:embed active.png
	IconActive []byte
	//go:embed waiting.png
	IconWaiting []byte
	//go:embed done.png
	IconDone []byte
	//go:embed transparent.png
	IconTransparent []byte
)
