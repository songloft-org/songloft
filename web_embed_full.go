//go:build full

package main

import (
	"embed"
)

//go:embed all:mimusic-player-build/web-embedded
var WebDist embed.FS
