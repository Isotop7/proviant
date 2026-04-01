package assets

import "embed"

//go:embed "css" "icons" "js" "fonts" "manifest.json"
var AssetFiles embed.FS
