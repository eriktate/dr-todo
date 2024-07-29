package drtodo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

func ParseDate(input string) (time.Time, error) {
	return time.Parse(dateLayout, input)
}

func FormatDate(date time.Time) string {
	return date.Format(dateLayout)
}

type Todo struct {
	Name      string
	Completed bool
}

func (t Todo) String() string {
	checkbox := "- [ ]"
	if t.Completed {
		checkbox = "- [x]"
	}

	return fmt.Sprintf("%s %s", checkbox, t.Name)
}

type List struct {
	Name     string
	Sublists []List
	Todos    []Todo
}

func (l List) Dump(w io.Writer, omitCompleted bool) error {
	fmt.Fprintf(w, "# %s\n", l.Name)
	for _, todo := range l.Todos {
		if !omitCompleted || !todo.Completed {
			fmt.Fprintln(w, todo.String())
		}
	}

	fmt.Fprintln(w, "")
	for _, list := range l.Sublists {
		if err := list.Dump(w, omitCompleted); err != nil {
			return fmt.Errorf("dumping sublist '%s': %w", list.Name, err)
		}
	}

	return nil
}

type DrTodo struct {
	home string
}

func New(homePath string) DrTodo {
	return DrTodo{
		home: homePath,
	}
}

func ParseTodo(input string) (Todo, error) {
	parts := strings.Split(input, "]")
	if len(parts) < 2 {
		return Todo{}, errors.New("missing checkbox")
	}

	checkbox := strings.ReplaceAll(parts[0]+"]", " ", "")
	checked := false
	switch checkbox {
	case "-[]":
		checked = false
	case "-[x]":
		checked = true
	default:
		return Todo{}, fmt.Errorf("invalid checkbox string '%s'", checkbox)
	}

	return Todo{
		Name:      strings.Trim(parts[1], " \n"),
		Completed: checked,
	}, nil
}

func getHeaderText(input string) string {
	return strings.Trim(input, " #\n")
}

func ParseList(date time.Time, reader io.Reader) (List, error) {
	r := bufio.NewReader(reader)

	list := List{
		Sublists: make([]List, 0, 10),
		Todos:    make([]Todo, 0, 100),
	}

	headerLine, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return List{}, fmt.Errorf("reading header: %w", err)
	}

	list.Name = getHeaderText(headerLine)

	for {
		rawLine, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			return List{}, fmt.Errorf("reading line: %w", err)
		}

		line := []rune(rawLine)
		if line[0] == '\n' {
			continue
		}

		if line[0] == '#' {
			listName := getHeaderText(string(line))
			if list.Name == "" {
				list.Name = listName
				continue
			}

			list.Sublists = append(list.Sublists, List{Name: listName})
			continue
		}

		todo, err := ParseTodo(rawLine)
		if err != nil {
			return List{}, fmt.Errorf("parsing todo: %w", err)
		}

		if len(list.Sublists) == 0 {
			list.Todos = append(list.Todos, todo)
			continue
		}

		idx := len(list.Sublists) - 1
		list.Sublists[idx].Todos = append(list.Sublists[idx].Todos, todo)
	}

	return list, nil
}

func GetLatestList(path string) (List, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return List{}, fmt.Errorf("reading dir: %w", err)
	}

	if len(entries) == 0 {
		return List{}, nil
	}

	latestDate := time.Time{}
	for _, entry := range entries {
		log.Printf("scanning dir: '%s'", entry.Name())
		parts := strings.Split(entry.Name(), ".")
		if len(parts) != 2 {
			return List{}, errors.New("file names can only contain a single '.'")
		}

		date, err := ParseDate(parts[0])
		if err != nil {
			return List{}, fmt.Errorf("parsing date: %w", err)
		}

		if date.Compare(latestDate) > 0 {
			latestDate = date
		}
	}

	file, err := os.Open(filepath.Join(path, FormatDate(latestDate)+".md"))
	if err != nil {
		return List{}, fmt.Errorf("opening latest file: %w", err)
	}
	defer file.Close()

	list, err := ParseList(latestDate, file)
	if err != nil {
		return List{}, fmt.Errorf("parsing list: %w", err)
	}

	return list, nil
}

func (dt DrTodo) CreateToday() (string, error) {
	if err := os.Mkdir(dt.home, fs.ModeDir|0777); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("creating dr-todo home directory '%s': %w", dt.home, err)
		}
	}

	today := FormatDate(time.Now())
	fname := today + ".md"
	path := path.Join(dt.home, fname)

	_, err := os.Stat(path)
	if err == nil {
		return "", fmt.Errorf("file '%s' already exists", fname)
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("checking if file exists: %w", err)
	}

	latest, err := GetLatestList(dt.home)
	if err != nil {
		return "", fmt.Errorf("getting previous list: %w", err)
	}

	latest.Name = fmt.Sprintf("TODO %s", today)
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating new todo file: %w", err)
	}
	defer file.Close()

	if err := latest.Dump(file, true); err != nil {
		return "", fmt.Errorf("dumping previous list: %w", err)
	}

	editor := os.Getenv("EDITOR")
	if editor != "" {
		log.Println("launching editor")
		cmd := exec.Command(editor, path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to start editor: %w", err)
		}
	}

	return fname, nil
}
