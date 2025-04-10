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
	"github.com/stretchr/testify/require"
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
			td, err := drtodo.ParseTodo("", c.todoStr)
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
	expected    string
	expectedErr error
}

func Test_ParseList(t *testing.T) {
	cases := []listCase{
		{
			name: "Valid list",
			listStr: `
# Test List
- [] Uncategorized

## Sublist
- [] Sublist todo
- [x] Completed sublist todo

# Must Do
- [] Do stuff
- [x] Do a thing

### Skipping a depth
- [] This should get reduced to a depth of 2

# Stretch Goals
- [] Do a stretch
`,
			expected: `# Test List
- [ ] Uncategorized

## Sublist
- [ ] Sublist todo

# Must Do
- [ ] Do stuff

## Skipping a depth
- [ ] This should get reduced to a depth of 2

# Stretch Goals
- [ ] Do a stretch
`,
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
			expectedErr: errors.New("parsing list: parsing todo: missing checkbox"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := bytes.NewReader([]byte(c.listStr))
			list, err := drtodo.ParseList(time.Now().String(), buf)
			if c.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, c.expectedErr.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			out := bytes.NewBuffer(nil)
			err = list.Dump(out, true)
			if c.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, c.expectedErr, err)
				return
			}

			require.Equal(t, c.expected, out.String())
		})
	}
}

func Test_GetLatestList(t *testing.T) {
	// SETUP
	dir := t.TempDir()
	now := time.Now()

	today := drtodo.FormatDate(now)
	file, err := os.Create(filepath.Join(dir, today+".md"))
	assert.NoError(t, err)
	defer file.Close()

	file, err = os.Create(filepath.Join(dir, drtodo.FormatDate(now.Add(-24*time.Hour))+".md"))
	assert.NoError(t, err)
	defer file.Close()

	file, err = os.Create(filepath.Join(dir, drtodo.FormatDate(now.Add(-48*time.Hour))+".md"))
	assert.NoError(t, err)
	defer file.Close()

	// RUN
	list, err := drtodo.GetLatestList(dir)
	assert.NoError(t, err)

	// ASSERT
	assert.Equal(t, today, list.Name)
}
