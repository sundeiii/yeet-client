package notifications

import (
	"bytes"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Windows shows toasts through a small PowerShell script. The script never
// changes: the texts (which can come from other people, like chat
// messages) are read from files, so nothing in them is ever run as code,
// and the files are UTF-8 so every character comes out right.
const toastScript = "\xef\xbb\xbf" + `param($appFile, $xmlFile)
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$appId = [IO.File]::ReadAllText($appFile, [Text.Encoding]::UTF8)
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml([IO.File]::ReadAllText($xmlFile, [Text.Encoding]::UTF8))
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appId).Show($toast)
`

func (n *Notification) Push() error {
	dir, err := os.MkdirTemp("", "yeet-toast-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	appId := n.Application
	if appId == "" {
		appId = "puush"
	}
	script := filepath.Join(dir, "toast.ps1")
	appFile := filepath.Join(dir, "app.txt")
	xmlFile := filepath.Join(dir, "toast.xml")
	for path, content := range map[string]string{
		script:  toastScript,
		appFile: appId,
		xmlFile: toastXML(n.Title, n.Text, n.iconPath, n.actionUrl, n.silent),
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}

	cmd := exec.Command("PowerShell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, appFile, xmlFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

// toastXML is the toast's content, with every text escaped.
func toastXML(title, text, iconPath, actionUrl string, silent bool) string {
	escape := func(value string) string {
		var out bytes.Buffer
		xml.EscapeText(&out, []byte(value))
		return out.String()
	}
	var out bytes.Buffer
	out.WriteString(`<toast activationType="protocol" duration="short"`)
	if actionUrl != "" {
		out.WriteString(` launch="` + escape(actionUrl) + `"`)
	}
	out.WriteString(`><visual><binding template="ToastGeneric">`)
	if iconPath != "" {
		out.WriteString(`<image placement="appLogoOverride" src="` + escape(iconPath) + `"/>`)
	}
	if title != "" {
		out.WriteString(`<text>` + escape(title) + `</text>`)
	}
	if text != "" {
		out.WriteString(`<text>` + escape(text) + `</text>`)
	}
	out.WriteString(`</binding></visual>`)
	if silent {
		out.WriteString(`<audio silent="true"/>`)
	}
	out.WriteString(`</toast>`)
	return out.String()
}
