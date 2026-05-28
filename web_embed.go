//go:build !full

package main

import (
	"embed"
)

// WebDist 轻量版本使用空 embed.FS
var WebDist embed.FS
