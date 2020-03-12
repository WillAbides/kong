package kong

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestFilesDir(t *testing.T) (teardown func()) {
	t.Helper()
	var err error
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	files := []string{
		"dir/foo",
		"dir/bar",
		"outer/inner/readme.md",
		".dot.txt",
		"a.txt",
		"b.txt",
		"c.txt",
		"readme.md",
	}
	for _, file := range files {
		file = filepath.Join(tmpDir, filepath.FromSlash(file))
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0700))
		require.NoError(t, ioutil.WriteFile(file, nil, 0600))
	}
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	return func() {
		require.NoError(t, os.Chdir(wd))
		require.NoError(t, os.RemoveAll(tmpDir))
	}
}

func TestCompleteDirs(t *testing.T) {
	teardown := setupTestFilesDir(t)
	defer teardown()
	for pattern, args := range map[string]map[string][]string{
		"*": {
			"di":     {"dir/"},
			"dir":    {"dir/"},
			"dir/":   {"dir/"},
			"./di":   {"./dir/"},
			"./dir":  {"./dir/"},
			"./dir/": {"./dir/"},
			"":       {"./", "dir/", "outer/"},
			".":      {"./", "./dir/", "./outer/"},
			"./":     {"./", "./dir/", "./outer/"},
		},
		"*.md": {
			"ou":       {"outer/", "outer/inner/"},
			"outer":    {"outer/", "outer/inner/"},
			"outer/":   {"outer/", "outer/inner/"},
			"./ou":     []string{"./outer/", "./outer/inner/"},
			"./outer":  []string{"./outer/", "./outer/inner/"},
			"./outer/": []string{"./outer/", "./outer/inner/"},
		},
		"dir": {
			"di":     {"dir/"},
			"dir":    {"dir/"},
			"dir/":   {"dir/"},
			"./di":   {"./dir/"},
			"./dir":  {"./dir/"},
			"./dir/": {"./dir/"},
		},
	} {
		pattern := pattern
		args := args
		t.Run(fmt.Sprintf("pattern:%q", pattern), func(t *testing.T) {
			completer := CompleteDirs()
			for arg, want := range args {
				arg := arg
				want := want
				t.Run(fmt.Sprintf("arg:%q", arg), func(t *testing.T) {
					got := completer.Options(newCompleterArgs(arg))
					sort.Strings(got)
					sort.Strings(want)
					require.Equal(t, want, got)
				})
			}
		})
	}
}

func TestCompleteFiles(t *testing.T) {
	teardown := setupTestFilesDir(t)
	defer teardown()
	for pattern, args := range map[string]map[string][]string{
		"*.txt": {
			"":       {"./", "dir/", "outer/", "a.txt", "b.txt", "c.txt", ".dot.txt"},
			"./dir/": []string{"./dir/"},
		},
		"*": {
			"./dir/f":   []string{"./dir/foo"},
			"./dir/foo": []string{"./dir/foo"},
			"dir":       []string{"dir/", "dir/foo", "dir/bar"},
			"di":        []string{"dir/", "dir/foo", "dir/bar"},
			"dir/":      []string{"dir/", "dir/foo", "dir/bar"},
			"./dir":     []string{"./dir/", "./dir/foo", "./dir/bar"},
			"./dir/":    []string{"./dir/", "./dir/foo", "./dir/bar"},
			"./di":      []string{"./dir/", "./dir/foo", "./dir/bar"},
		},
		"*.md": {
			"":        []string{"./", "dir/", "outer/", "readme.md"},
			".":       []string{"./", "./dir/", "./outer/", "./readme.md"},
			"./":      []string{"./", "./dir/", "./outer/", "./readme.md"},
			"outer/i": []string{"outer/inner/", "outer/inner/readme.md"},
		},
		"foo": {
			"./dir/": []string{"./dir/", "./dir/foo"},
			"./d":    []string{"./dir/", "./dir/foo"},
		},
	} {
		pattern := pattern
		args := args
		t.Run(fmt.Sprintf("pattern:%q", pattern), func(t *testing.T) {
			completer := CompleteFiles(pattern)
			for arg, want := range args {
				arg := arg
				want := want
				t.Run(fmt.Sprintf("arg:%q", arg), func(t *testing.T) {
					got := completer.Options(newCompleterArgs(arg))
					sort.Strings(got)
					sort.Strings(want)
					require.Equal(t, want, got)
				})
			}
		})
	}
}

func TestPositionalCompleter_position(t *testing.T) {
	posCompleter := &positionalCompleter{
		Flags: []*Flag{
			{
				Value: &Value{
					Name:   "mybool",
					Mapper: boolMapper{},
				},
				Short: 'b',
			},
			{
				Value: &Value{
					Name:   "mybool2",
					Mapper: boolMapper{},
				},
				Short: 'c',
			},
			{
				Value: &Value{
					Name: "myarg",
				},
				Short: 'a',
			},
		},
	}

	for args, want := range map[string]int{
		``:                 0,
		`foo`:              0,
		`foo `:             1,
		`-b foo `:          1,
		`-bc foo `:         1,
		`-bd foo `:         1,
		`-a foo `:          0,
		`-a=omg foo `:      1,
		`--myarg omg foo `: 1,
		`--myarg=omg foo `: 1,
		`foo bar`:          1,
		`foo bar `:         2,
	} {
		args := args
		want := want
		t.Run(args, func(t *testing.T) {
			got := posCompleter.completerIndex(newCompleterArgs("foo " + args))
			assert.Equal(t, want, got)
		})
	}
}

func TestPositionalCompleter_Predict(t *testing.T) {
	completer1 := CompleteSet("1")
	completer2 := CompleteSet("2")
	posCompleter := &positionalCompleter{
		Completers: []Completer{completer1, completer2},
	}

	for args, want := range map[string][]string{
		``:         {"1"},
		`foo`:      {"1"},
		`foo `:     {"2"},
		`foo bar`:  {"2"},
		`foo bar `: {},
	} {
		args := args
		want := want
		t.Run(args, func(t *testing.T) {
			got := posCompleter.Options(newCompleterArgs("app " + args))

			assert.Equal(t, want, got)
		})
	}
}

func setLineAndPoint(t *testing.T, line string, point *int) func() {
	pVal := len(line)
	if point != nil {
		pVal = *point
	}
	const (
		envLine  = "COMP_LINE"
		envPoint = "COMP_POINT"
	)
	t.Helper()
	origLine, hasOrigLine := os.LookupEnv(envLine)
	origPoint, hasOrigPoint := os.LookupEnv(envPoint)
	require.NoError(t, os.Setenv(envLine, line))
	require.NoError(t, os.Setenv(envPoint, strconv.Itoa(pVal)))
	return func() {
		t.Helper()
		require.NoError(t, os.Unsetenv(envLine))
		require.NoError(t, os.Unsetenv(envPoint))
		if hasOrigLine {
			require.NoError(t, os.Setenv(envLine, origLine))
		}
		if hasOrigPoint {
			require.NoError(t, os.Setenv(envPoint, origPoint))
		}
	}
}

func TestComplete(t *testing.T) {
	type embed struct {
		Lion string
	}

	completers := Completers{
		"things":      CompleteSet("thing1", "thing2"),
		"otherthings": CompleteSet("otherthing1", "otherthing2"),
	}

	var cli struct {
		Foo struct {
			Embedded embed  `kong:"embed"`
			Bar      string `kong:"completer=things"`
			Baz      bool
			Rabbit   struct {
			} `kong:"cmd"`
			Duck struct {
			} `kong:"cmd"`
		} `kong:"cmd"`
		Bar struct {
			Tiger   string `kong:"arg,completer=things"`
			Bear    string `kong:"arg,completer=otherthings"`
			OMG     string `kong:"enum='oh,my,gizzles'"`
			Number  int    `kong:"short=n,enum='1,2,3'"`
			BooFlag bool   `kong:"name=boofl,short=b"`
		} `kong:"cmd"`
	}

	type completeTest struct {
		want  []string
		line  string
		point *int
	}

	lenPtr := func(val string) *int {
		v := len(val)
		return &v
	}

	tests := []completeTest{
		{
			line: "myApp ",
			want: []string{"bar", "foo"},
		},
		{
			line: "myApp foo",
			want: []string{"foo"},
		},
		{
			line: "myApp foo ",
			want: []string{"duck", "rabbit"},
		},
		{
			line: "myApp foo r",
			want: []string{"rabbit"},
		},
		{
			line: "myApp -",
			want: []string{"--help"},
		},
		{
			line: "myApp foo -",
			want: []string{"--bar", "--baz", "--help", "--lion"},
		},
		{
			line: "myApp foo --lion ",
			want: []string{},
		},
		{
			line: "myApp foo --baz ",
			want: []string{"duck", "rabbit"},
		},
		{
			line: "myApp foo --baz -",
			want: []string{"--bar", "--baz", "--help", "--lion"},
		},
		{
			line: "myApp foo --bar ",
			want: []string{"thing1", "thing2"},
		},
		{
			line: "myApp bar ",
			want: []string{"thing1", "thing2"},
		},
		{
			line: "myApp bar thing",
			want: []string{"thing1", "thing2"},
		},
		{
			line: "myApp bar thing1 ",
			want: []string{"otherthing1", "otherthing2"},
		},
		{
			line: "myApp bar --omg ",
			want: []string{"gizzles", "my", "oh"},
		},
		{
			line: "myApp bar -",
			want: []string{"--boofl", "--help", "--number", "--omg", "-b", "-n"},
		},
		{
			line: "myApp bar -b ",
			want: []string{"thing1", "thing2"},
		},
		{
			line: "myApp bar -b thing1 -",
			want: []string{"--boofl", "--help", "--number", "--omg", "-b", "-n"},
		},
		{
			line: "myApp bar -b thing1 --omg ",
			want: []string{"gizzles", "my", "oh"},
		},
		{
			line: "myApp bar -b thing1 --omg gizzles ",
			want: []string{"otherthing1", "otherthing2"},
		},
		{
			line: "myApp bar -b thing1 --omg gizzles ",
			want: []string{"otherthing1", "otherthing2"},
		},
		{
			line: "myApp bar -b thing1 --omg gi",
			want: []string{"gizzles"},
		},
		{
			line:  "myApp bar -b thing1 --omg gi",
			want:  []string{"thing1", "thing2"},
			point: lenPtr("myApp bar -b th"),
		},
		{
			line:  "myApp bar -b thing1 --omg gizzles ",
			want:  []string{"thing1", "thing2"},
			point: lenPtr("myApp bar -b th"),
		},
		{
			line:  "myApp bar -b thing1 --omg gizzles ",
			want:  []string{"thing1"},
			point: lenPtr("myApp bar -b thing1"),
		},
		{
			line:  "myApp bar -b thing1 --omg gizzles ",
			want:  []string{"otherthing1", "otherthing2"},
			point: lenPtr("myApp bar -b thing1 "),
		},
		{
			line: "myApp bar --number ",
			want: []string{"1", "2", "3"},
		},
		{
			line: "myApp bar --number=",
			want: []string{"1", "2", "3"},
		},
	}

	for _, td := range tests {
		td := td
		t.Run(td.line, func(t *testing.T) {
			var stdOut, stdErr bytes.Buffer
			var exited bool
			p, err := New(&cli,
				Writers(&stdOut, &stdErr),
				Exit(func(i int) {
					exited = assert.Equal(t, 0, i)
				}),
				Name("test"),
				completers,
			)
			require.NoError(t, err)
			cleanup := setLineAndPoint(t, td.line, td.point)
			defer cleanup()
			_, err = p.Parse([]string{})
			require.Error(t, err)
			require.IsType(t, &ParseError{}, err)
			require.True(t, exited)
			require.Equal(t, "", stdErr.String())
			gotLines := strings.Split(stdOut.String(), "\n")
			sort.Strings(gotLines)
			gotOpts := []string{}
			for _, l := range gotLines {
				if l != "" {
					gotOpts = append(gotOpts, l)
				}
			}
			require.Equal(t, td.want, gotOpts)
		})
	}
}
