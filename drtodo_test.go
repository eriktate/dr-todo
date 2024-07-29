package drtodo_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	drtodo "github.com/eriktate/dr-todo"
	"github.com/stretchr/testify/assert"
)

type todoCase struct {
	name        string
	todoStr     string
	expected    drtodo.Todo
	expectedErr error
}

func Test_ParseTodo(t *testing.T) {
	cases := []todoCase{
		{
			name:    "Unfinished TODO",
			todoStr: "- [] Test TODO",
			expected: drtodo.Todo{
				Name:      "Test TODO",
				Completed: false,
			},
		},
		{
			name:    "Finished TODO",
			todoStr: "- [x] Test TODO",
			expected: drtodo.Todo{
				Name:      "Test TODO",
				Completed: true,
			},
		},
		{
			name:        "Missing checkbox",
			todoStr:     "- Test TODO",
			expectedErr: errors.New("missing checkbox"),
		},
		{
			name:        "Invalid checkbox",
			todoStr:     "- [o] Test TODO",
			expectedErr: errors.New("invalid checkbox string '-[o]'"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			td, err := drtodo.ParseTodo(c.todoStr)
			if c.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, c.expectedErr.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.expected, td)
		})
	}
}

type listCase struct {
	name        string
	listStr     string
	expected    drtodo.List
	expectedErr error
}

func Test_ParseList(t *testing.T) {
	cases := []listCase{
		{
			name: "Valid list",
			listStr: `# Test List
- [] Uncategorized

# Must Do
- [] Do stuff
- [x] Do a thing

# Stretch Goals
- [] Do a stretch
`,
			expected: drtodo.List{
				Name: "Test List",
				Todos: []drtodo.Todo{
					{
						Name:      "Uncategorized",
						Completed: false,
					},
				},
				Sublists: []drtodo.List{
					{
						Name: "Must Do",
						Todos: []drtodo.Todo{
							{
								Name:      "Do stuff",
								Completed: false,
							},
							{
								Name:      "Do a thing",
								Completed: true,
							},
						},
					},
					{
						Name: "Stretch Goals",
						Todos: []drtodo.Todo{
							{
								Name:      "Do a stretch",
								Completed: false,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "Invalid list",
			listStr: `
$ Must Do
- [] Do stuff
- [x] Do a thing

$ Stretch Goals
- [] Do stretch things
			`,
			expectedErr: errors.New("parsing todo: missing checkbox"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := bytes.NewReader([]byte(c.listStr))
			list, err := drtodo.ParseList(time.Now(), buf)
			if c.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, c.expectedErr.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			for idx, expected := range c.expected.Sublists {
				actual := list.Sublists[idx]
				assert.Equal(t, expected.Name, actual.Name)
				assert.ElementsMatch(t, expected.Todos, actual.Todos)
				assert.ElementsMatch(t, expected.Sublists, actual.Sublists)
			}
		})
	}
}

func Test_GetLatestList(t *testing.T) {
	// SETUP
	dir := t.TempDir()
	now := time.Now()

	file, err := os.Create(filepath.Join(dir, drtodo.FormatDate(now)+".md"))
	assert.NoError(t, err)
	file.WriteString("# Latest")
	file.Close()

	file, err = os.Create(filepath.Join(dir, drtodo.FormatDate(now.Add(-24*time.Hour))+".md"))
	assert.NoError(t, err)
	file.WriteString("# Second Latest")
	file.Close()

	file, err = os.Create(filepath.Join(dir, drtodo.FormatDate(now.Add(-48*time.Hour))+".md"))
	assert.NoError(t, err)
	file.WriteString("# Third Latest")
	file.Close()

	// RUN
	list, err := drtodo.GetLatestList(dir)
	assert.NoError(t, err)

	// ASSERT
	assert.Equal(t, "Latest", list.Name)
}
