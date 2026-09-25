package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	appipc "github.com/sundeiii/yeet-client/internal/ipc"
)

func parseIpcCommand(arguments []string) (appipc.Command, error) {
	flags := flag.NewFlagSet("puush", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	upload := flags.Bool("upload", false, "upload one or more files")
	choose := flags.Bool("choose", false, "open an interactive window that lets you choose a file to upload")
	toggle := flags.Bool("toggle", false, "toggle puush shortcuts on or off")

	if err := flags.Parse(arguments); err != nil {
		return appipc.Command{}, err
	}
	if *choose {
		return appipc.NewChooseFileCommand(), nil
	}
	if *toggle {
		return appipc.NewToggleShortcutsCommand(), nil
	}
	if *upload {
		return appipc.NewUploadCommand(flags.Args())
	}
	if len(flags.Args()) > 0 {
		return appipc.Command{}, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if len(arguments) > 0 {
		return appipc.Command{}, errors.New("no command was selected")
	}
	return appipc.NewAttentionCommand(), nil
}
