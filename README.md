# yeet-client

The desktop app for [yeet](https://p.tupsujumal.ee): take a screenshot or pick a file, and a short link lands in your clipboard. Works on Windows, macOS and Linux.

It speaks the classic puush API, so it also works with other puush-compatible servers. Change the server in **Settings → Advanced**.

## Features

- Screenshots of an area, a window or the whole screen, with global shortcuts
- Upload any file, the clipboard, or right-click a file in your file manager
- Recent uploads in the tray menu, notifications with a sound, links copied automatically
- Starts with your computer and updates itself from this repository's releases

Default shortcuts: area `Ctrl+Shift+4`, full screen `Ctrl+Shift+3`, window `Ctrl+Shift+2`, clipboard `Ctrl+Shift+5`, file `Ctrl+Shift+U`, and `Ctrl+Alt+P` to pause and resume all of them.

## Installation

Download the file for your system from the [latest release](https://github.com/sundeiii/yeet-client/releases/latest) and put it somewhere you can write to, so it can update itself:

- **Windows**: `yeet-windows-amd64.exe`, e.g. in `%LOCALAPPDATA%\yeet`
- **macOS**: `yeet-macos-arm64.app.zip` (Apple silicon) or `yeet-macos-amd64.app.zip` (Intel), unzipped into `~/Applications`
- **Linux**: `yeet-linux-amd64` or `yeet-linux-arm64`, e.g. in `~/.local/bin` (make it executable with `chmod +x`)

Windows may show a SmartScreen warning because the app isn't code-signed: click **More info → Run anyway**.

On macOS, remove the download quarantine or the system reports the app as damaged:

```sh
xattr -d com.apple.quarantine yeet-macos-arm64.app
```

### Linux dependencies

- Screenshots: `spectacle` (KDE) or `gnome-screenshot` (GNOME) work best; `flameshot`, `maim` and `grim`/`slurp` also work
- Notifications and sound: `notify-send` and `paplay`
- Clipboard on Wayland: `wl-clipboard`
- Right-click uploads: Nautilus, Dolphin and Nemo are supported

## Building

```bash
go build -o yeet ./cmd/desktop
```

The app uses [Fyne](https://fyne.io/), which needs a C compiler and a few system libraries. See [Fyne's prerequisites](https://docs.fyne.io/started/quick/). GitHub Actions builds every platform on each push; publishing a release attaches the binaries, and installed apps pick them up automatically.

To point a build at a different default server:

```bash
go build -ldflags "-X github.com/sundeiii/yeet-client/internal/config.DefaultServerURL=https://files.example.com" -o yeet ./cmd/desktop
```

## Using the API from Go

`pkg/puush` is a client for the puush API:

```go
client := puush.NewClientFromLogin("you@example.com", "password")
client.SetBaseURL("https://p.tupsujumal.ee")
if err := client.Authenticate(); err != nil {
	panic(err)
}

file, _ := os.Open("example.png")
defer file.Close()
url, err := client.Upload(file)
```

## License

MIT, see [LICENSE](LICENSE).
