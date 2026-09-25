package ipc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const maxUploadPaths = 255

type Action uint8

const (
	ActionAttention Action = iota + 1
	ActionUpload
	ActionChooseFile
	ActionToggleShortcuts
)

var (
	ErrTooManyUploadPaths = fmt.Errorf("cannot upload more than %d files at once", maxUploadPaths)
	ErrNoUploadPaths      = errors.New("no upload paths were provided")
	ErrUnsupportedAction  = errors.New("unsupported IPC action")
)

// Command contains the operation requested by a newly launched process.
type Command struct {
	Action      Action
	UploadPaths []string
}

// NewAttentionCommand creates a command that asks the primary process to
// bring its relevant window to the foreground.
func NewAttentionCommand() Command {
	return Command{Action: ActionAttention}
}

// NewUploadCommand creates an upload command with normalized absolute paths.
func NewUploadCommand(paths []string) (Command, error) {
	if len(paths) == 0 {
		return Command{}, ErrNoUploadPaths
	}
	if len(paths) > maxUploadPaths {
		return Command{}, ErrTooManyUploadPaths
	}

	absolutePaths := make([]string, len(paths))
	for i, path := range paths {
		if path == "" {
			return Command{}, errors.New("upload path cannot be empty")
		}
		absolutePath, err := filepath.Abs(path)
		if err != nil {
			return Command{}, fmt.Errorf("resolve upload path %q: %w", path, err)
		}
		absolutePaths[i] = filepath.Clean(absolutePath)
	}

	return Command{Action: ActionUpload, UploadPaths: absolutePaths}, nil
}

// NewChooseFileCommand creates a command that allows the user to pick a file to upload.
func NewChooseFileCommand() Command {
	return Command{Action: ActionChooseFile}
}

// NewToggleShortcutsCommand creates a command that toggles puush shortcuts.
func NewToggleShortcutsCommand() Command {
	return Command{Action: ActionToggleShortcuts}
}

// Validate checks whether the command's action and payload valid.
func (command Command) Validate() error {
	switch command.Action {
	case ActionAttention:
		if len(command.UploadPaths) != 0 {
			return errors.New("attention command cannot contain upload paths")
		}
		return nil
	case ActionChooseFile:
		if len(command.UploadPaths) != 0 {
			return errors.New("choose command cannot contain upload paths")
		}
		return nil
	case ActionToggleShortcuts:
		if len(command.UploadPaths) != 0 {
			return errors.New("toggle command cannot contain upload paths")
		}
		return nil
	case ActionUpload:
		if len(command.UploadPaths) == 0 {
			return ErrNoUploadPaths
		}
		if len(command.UploadPaths) > maxUploadPaths {
			return ErrTooManyUploadPaths
		}
		if slices.Contains(command.UploadPaths, "") {
			return errors.New("upload path cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("%w: %d", ErrUnsupportedAction, command.Action)
	}
}

// ValidateReceived treats the command as untrusted RPC input.
func (command Command) ValidateReceived() (Command, error) {
	if err := command.Validate(); err != nil {
		return Command{}, err
	}

	switch command.Action {
	case ActionAttention:
		return NewAttentionCommand(), nil
	case ActionChooseFile:
		return NewChooseFileCommand(), nil
	case ActionToggleShortcuts:
		return NewToggleShortcutsCommand(), nil
	case ActionUpload:
		paths, err := validatePaths(command.UploadPaths)
		if err != nil {
			return Command{}, err
		}
		return Command{Action: ActionUpload, UploadPaths: paths}, nil
	}
	return Command{}, ErrUnsupportedAction
}

func validatePaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, ErrNoUploadPaths
	}
	if len(paths) > maxUploadPaths {
		return nil, ErrTooManyUploadPaths
	}

	validated := make([]string, len(paths))
	for i, path := range paths {
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("upload path must be absolute: %q", path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect upload path %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("upload path is not a regular file: %q", path)
		}
		validated[i] = filepath.Clean(path)
	}
	return validated, nil
}
