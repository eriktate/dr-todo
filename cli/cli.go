package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strconv"

	drtodo "github.com/eriktate/dr-todo"
	"github.com/urfave/cli/v2"
)

var homePath string

func HandleNew() *cli.Command {
	var skipEdit bool

	return &cli.Command{
		Name:        "new",
		Usage:       "Create a new list for today",
		Description: "Prints an error if the list already exists. If the --edit flag is provided, attempts to open $EDITOR regardless of error response",
		UsageText:   "dr-todo [global options] new [--skip-edit]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "skip-edit",
				Value:       false,
				Usage:       "Skips opening the TODO file with $EDITOR after creation",
				Destination: &skipEdit,
			},
		},
		Action: func(ctx *cli.Context) error {
			dt := drtodo.New(homePath)
			path, err := dt.CreateToday()
			if err != nil {
				return fmt.Errorf("failed to create new list: %w", err)
			}

			fmt.Fprintf(ctx.App.Writer, "%s created ✅\n", path)

			editor := os.Getenv("EDITOR")
			if !skipEdit {
				if editor == "" {
					return errors.New("could not open list in $EDITOR because it isn't set")
				}

				cmd := exec.Command(editor, path)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin

				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to start editor '%s': %w", editor, err)
				}
			}

			return nil
		},
	}
}

func HandleEdit() *cli.Command {
	return &cli.Command{
		Name:        "edit",
		Usage:       "Opens a list in $EDITOR",
		Description: "By default opens the latest list. Previous lists can be opened by providing an offset which counts backwards",
		UsageText:   "dr-todo [global options] edit [offset]",
		Action: func(ctx *cli.Context) error {
			var err error
			offset := 0
			offsetStr := ctx.Args().First()
			if offsetStr != "" {
				offset, err = strconv.Atoi(offsetStr)
				if err != nil {
					return fmt.Errorf("provided offset is invalid, '%s' must be a positive integer: %w", offsetStr, err)
				}
			}

			paths, err := drtodo.GetSortedListPaths(homePath)
			if err != nil {
				return fmt.Errorf("could not find latest list")
			}

			if len(paths) == 0 {
				return fmt.Errorf("no lists found in %s", homePath)
			}

			if offset >= len(paths) {
				offset = len(paths) - 1
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				return errors.New("could not open list in $EDITOR because it isn't set")
			}

			if editor != "" {
				cmd := exec.Command(editor, paths[offset])
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin

				if err := cmd.Run(); err != nil {
					return fmt.Errorf("could not start editor '%s': %w", editor, err)
				}
			}

			return nil
		},
	}
}

func Run() error {
	fallbackHome := os.Getenv("DRTODO_HOME")
	if fallbackHome == "" {
		fallbackHome = path.Join(os.Getenv("HOME"), ".todos") // default
	}

	app := &cli.App{
		Name: "dr-todo",
		Authors: []*cli.Author{
			{
				Name:  "Erik Tate",
				Email: "hello@eriktate.me",
			},
		},
		Usage: "Simple creation of daily TODO files carrying over tasks from previous days",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "home",
				Value:       fallbackHome,
				DefaultText: "~/.todos",
				Usage:       "sets home directory dr-todo should parse and save todos to",
				Destination: &homePath,
			},
		},
		Before: func(ctx *cli.Context) error {
			if homePath == "" {
				return errors.New("no valid home path could be found")
			}

			if err := os.Mkdir(homePath, fs.ModeDir|0777); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return fmt.Errorf("creating dr-todo home directory '%s': %w", homePath, err)
				}
			}

			return nil
		},
		DefaultCommand: "edit",
		Commands: []*cli.Command{
			HandleNew(),
			HandleEdit(),
		},
	}

	return app.Run(os.Args)
}
