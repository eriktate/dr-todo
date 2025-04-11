package drtodo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/eriktate/go-ordmap"
)

const dateLayout = "2006-01-02"

var ErrListNotFound = errors.New("list not found")

func ParseDate(input string) (time.Time, error) {
	return time.Parse(dateLayout, input)
}

func FormatDate(date time.Time) string {
	return date.Format(dateLayout)
}

var todoID = math.MaxInt32

type Todo struct {
	ListID    string
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
	Sublists *ordmap.OrdMap[string, []Todo]
}

func getHeaderWithDepth(input string) (string, int) {
	var depth int
	for idx, char := range input {
		if char != '#' {
			depth = idx
			break
		}
	}

	return strings.Trim(input, " #\n"), depth
}

const listSep = ":@>"

type listName struct {
	name  string
	depth int
}

func parseList(reader io.Reader) (List, error) {
	r := bufio.NewReader(reader)
	listStack := make([]listName, 0, 10)

	sublists := ordmap.NewUnsafe[string, []Todo](10)

	for {
		rawLine, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return List{}, fmt.Errorf("reading line: %w", err)
		}

		line := []rune(rawLine)
		if line[0] == '\n' {
			continue
		}

		if line[0] == '#' {
			depth := 0
			if len(listStack) > 0 {
				depth = listStack[len(listStack)-1].depth
			}
			name, d := getHeaderWithDepth(rawLine)
			ln := listName{name: name, depth: d}
			switch {
			case d > depth:
				listStack = append(listStack, ln)
			case d == depth:
				listStack[len(listStack)-1] = ln
			case d < depth:
				for len(listStack) > 0 && listStack[len(listStack)-1].depth >= d {
					listStack = listStack[:len(listStack)-1]
				}
				listStack = append(listStack, ln)
			}

			continue
		}

		listNames := make([]string, len(listStack))
		for idx, ln := range listStack {
			listNames[idx] = ln.name
		}

		listID := strings.Join(listNames, listSep)
		todo, err := ParseTodo(listID, rawLine)
		if err != nil {
			return List{}, fmt.Errorf("parsing todo: %w", err)
		}

		todos, _ := sublists.Get(listID)
		sublists.Set(listID, append(todos, todo))
	}

	return List{Sublists: sublists}, nil
}

func (l List) Dump(w io.Writer, omitCompleted bool) error {
	firstLine := true
	for listID, todos := range l.Sublists.EntryIter() {
		parts := strings.Split(listID, listSep)
		if !firstLine {
			fmt.Fprint(w, "\n")
		}
		firstLine = false
		hashes := strings.Repeat("#", len(parts))
		if _, err := fmt.Fprintf(w, "%s %s\n", hashes, parts[len(parts)-1]); err != nil {
			return fmt.Errorf("writing heading: %w", err)
		}

		for _, todo := range todos {
			if !omitCompleted || !todo.Completed {
				if _, err := fmt.Fprintln(w, todo.String()); err != nil {
					return fmt.Errorf("writing todo: %w", err)
				}
			}
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

func ParseTodo(listID, input string) (Todo, error) {
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
		ListID:    listID,
		Name:      strings.Trim(parts[1], " \n"),
		Completed: checked,
	}, nil
}

type listResult struct {
	list      List
	nextName  string
	nextDepth int
}

func ParseList(name string, reader io.Reader) (List, error) {
	res, err := parseList(reader)
	if err != nil {
		return List{}, fmt.Errorf("parsing list: %w", err)
	}

	res.Name = name
	return res, nil
}

func GetLatestList(path string) (List, error) {
	paths, err := GetSortedListPaths(path)
	if err != nil {
		return List{}, fmt.Errorf("getting sorted paths: %w", err)
	}

	if len(paths) == 0 {
		return List{}, ErrListNotFound
	}

	file, err := os.Open(paths[0])
	if err != nil {
		return List{}, fmt.Errorf("opening latest file: %w", err)
	}
	defer file.Close()

	listName := strings.TrimSuffix(filepath.Base(file.Name()), filepath.Ext(file.Name()))
	list, err := ParseList(listName, file)
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

	dates := []time.Time{}
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
		return b.Compare(a)
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
		if err != ErrListNotFound {
			return "", fmt.Errorf("getting previous list: %w", err)
		}
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
