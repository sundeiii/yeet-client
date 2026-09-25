package assets

import _ "embed"

//go:embed icons/puush.png
var PuushIconData []byte

//go:embed icons/puush-error.png
var PuushErrorIconData []byte // NOTE: Not part of the original client

//go:embed sounds/success.wav
var SuccessSoundData []byte

//go:embed icons/icon-window.png
var WindowIconData []byte

//go:embed icons/icon-fullscreen.png
var FullscreenIconData []byte

//go:embed icons/icon-upload.png
var UploadIconData []byte

//go:embed icons/icon-selection.png
var SelectionIconData []byte

//go:embed icons/icon-clipboard.png
var ClipboardIconData []byte // NOTE: Not part of the original client

//go:embed quickstart.png
var QuickstartData []byte // TODO: Remove "windows" text from quickstart asset

// Flags for the language picker, from osu! (see flags/CREDITS.txt)
//
//go:embed flags/GB.png
var FlagGBData []byte

//go:embed flags/NL.png
var FlagNLData []byte

//go:embed flags/DE.png
var FlagDEData []byte

// Flags maps the flag names of the languages to their pictures.
var Flags = map[string][]byte{"GB": FlagGBData, "NL": FlagNLData, "DE": FlagDEData}
