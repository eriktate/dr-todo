package drtodo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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

func ParseList(name string, reader io.Reader) (List, error) {
	r := bufio.NewReader(reader)

	list := List{
		Name:     name,
		Sublists: make([]List, 0, 10),
		Todos:    make([]Todo, 0, 100),
	}

	headerLine, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return List{}, fmt.Errorf("reading header: %w", err)
	}

	if list.Name == "" {
		list.Name = getHeaderText(headerLine)
	}

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
	paths, err := GetSortedListPaths(path)
	if err != nil {
		return List{}, fmt.Errorf("getting sorted paths: %w", err)
	}

	file, err := os.Open(paths[0])
	if err != nil {
		return List{}, fmt.Errorf("opening latest file: %w", err)
	}
	defer file.Close()

	list, err := ParseList("", file)
	if err != nil {
		return List{}, fmt.Errorf("parsing list: %w", err)
	}

	return list, nil
}

func GetSortedListPaths(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading dir: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	var dates []time.Time
	for _, entry := range entries {
		parts := strings.Split(entry.Name(), ".")
		if len(parts) != 2 {
			return nil, errors.New("file names can only contain a single '.'")
		}

		date, err := ParseDate(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parsing date: %w", err)
		}

		dates = append(dates, date)
	}

	slices.SortFunc(dates, func(a time.Time, b time.Time) int {
		return a.Compare(b) * -1
	})

	paths := make([]string, len(dates))
	for idx, date := range dates {
		paths[idx] = filepath.Join(path, FormatDate(date)+".md")
	}

	return paths, nil
}

func (dt DrTodo) CreateToday() (string, error) {

	today := FormatDate(time.Now())
	fname := today + ".md"
	path := filepath.Join(dt.home, fname)

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

	return path, nil
}
