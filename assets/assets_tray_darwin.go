package assets

import _ "embed"

//go:embed tray/osx/status-icon@2x.png
var TrayIconData []byte

//go:embed tray/osx/complete@2x.png
var TrayCompleteIconData []byte

//go:embed tray/osx/status-icon@2x.png
var TrayFailIconData []byte // TODO: Fail icon doesn't exist

//go:embed tray/osx/progress0@2x.png
var TrayProgress0IconData []byte

//go:embed tray/osx/progress10@2x.png
var TrayProgress10IconData []byte

//go:embed tray/osx/progress20@2x.png
var TrayProgress20IconData []byte

//go:embed tray/osx/progress30@2x.png
var TrayProgress30IconData []byte

//go:embed tray/osx/progress40@2x.png
var TrayProgress40IconData []byte

//go:embed tray/osx/progress50@2x.png
var TrayProgress50IconData []byte

//go:embed tray/osx/progress60@2x.png
var TrayProgress60IconData []byte

//go:embed tray/osx/progress70@2x.png
var TrayProgress70IconData []byte

//go:embed tray/osx/progress80@2x.png
var TrayProgress80IconData []byte

//go:embed tray/osx/progress90@2x.png
var TrayProgress90IconData []byte

//go:embed tray/osx/progress100@2x.png
var TrayProgress100IconData []byte
