package assets

import "embed"

//go:embed "css" "icons" "js" "fonts"
var AssetFiles embed.FS
