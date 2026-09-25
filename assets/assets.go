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
