package webui

import "embed"

//go:embed index.html styles.css app.js
var assets embed.FS

func ReadFile(name string) ([]byte, error) {
	return assets.ReadFile(name)
}
