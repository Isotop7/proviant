package assets

import "embed"

//go:embed "css" "icons" "js"
var AssetFiles embed.FS
