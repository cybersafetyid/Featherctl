package wiring_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/wiring"
)

const (
	routerMarker  = "// feather:register-routes (do not remove this comment)"
	fieldsMarker  = "// feather:container-fields (do not remove this comment)"
	wiringMarker  = "// feather:container-wiring (do not remove this comment)"
	featureImport = "example.com/demo/internal/features/order"
)

const routerTemplate = `package router

import (
	"net/http"

	"example.com/demo/internal/platform/container"
)

// New builds the HTTP router for the application.
func New(c *container.Container) *http.ServeMux {
	mux := http.NewServeMux()

	` + routerMarker + `

	return mux
}
`

const containerTemplate = `package container

import (
	"log/slog"
	"net/http"
)

// Container holds the application's wired dependencies.
type Container struct {
	` + fieldsMarker + `
	// Logger is the structured logger shared by the application.
	Logger *slog.Logger
}

// New constructs the application container.
func New() *Container {
	c := &Container{
		Logger: slog.Default(),
	}

	` + wiringMarker + `

	return c
}
`

// project creates a throwaway module so goimports can resolve local paths.
func project(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.22\n")
	write(t, filepath.Join(root, "internal/platform/router/router.go"), routerTemplate)
	write(t, filepath.Join(root, "internal/platform/container/container.go"), containerTemplate)
	write(t, filepath.Join(root, "internal/features/order/order.go"), "package order\n")
	return root
}

// write creates a file and its parents.
func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// read loads a file from disk.
func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// requireParses asserts that src is valid Go.
func requireParses(t *testing.T, src []byte) {
	t.Helper()
	_, err := parser.ParseFile(token.NewFileSet(), "generated.go", src, parser.ParseComments)
	require.NoError(t, err, "generated source must parse:\n%s", src)
}

// lineIndex returns the index of the first line equal to want, or -1.
func lineIndex(src, want string) int {
	for i, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == strings.TrimSpace(want) {
			return i
		}
	}
	return -1
}

func TestInjectStatementsAboveMarker(t *testing.T) {
	t.Parallel()

	root := project(t)
	path := filepath.Join(root, "internal/platform/router/router.go")

	out, err := wiring.Inject(read(t, path), wiring.Injection{
		Path:   path,
		Marker: routerMarker,
		Code:   "c.Order.RegisterRoutes(mux)",
	})
	require.NoError(t, err)
	requireParses(t, out)

	text := string(out)
	assert.Contains(t, text, "c.Order.RegisterRoutes(mux)")
	assert.Contains(t, text, routerMarker)
	assert.Less(t, lineIndex(text, "c.Order.RegisterRoutes(mux)"), lineIndex(text, routerMarker),
		"generated code is inserted above the marker")
	assert.Contains(t, text, "// New builds the HTTP router for the application.",
		"existing comments are preserved")
	assert.NotContains(t, text, featureImport, "no import needed for router")
}

func TestInjectStatementsAddsImport(t *testing.T) {
	t.Parallel()

	root := project(t)
	path := filepath.Join(root, "internal/platform/container/container.go")

	code := strings.Join([]string{
		"orderRepo := order.NewInMemoryRepository()",
		"orderService := order.NewService(orderRepo)",
		"c.Order = order.NewHandler(orderService)",
	}, "\n")

	out, err := wiring.Inject(read(t, path), wiring.Injection{
		Path:    path,
		Marker:  wiringMarker,
		Code:    code,
		Imports: []string{featureImport},
	})
	require.NoError(t, err)
	requireParses(t, out)

	text := string(out)
	assert.Contains(t, text, "c.Order = order.NewHandler(orderService)")
	assert.Contains(t, text, `"`+featureImport+`"`)
	assert.Less(t, lineIndex(text, "orderRepo := order.NewInMemoryRepository()"), lineIndex(text, wiringMarker))
	assert.Contains(t, text, wiringMarker)
	assert.Contains(t, text, fieldsMarker, "an unrelated marker is untouched")
}

func TestInjectFieldsBeforeMarker(t *testing.T) {
	t.Parallel()

	root := project(t)
	path := filepath.Join(root, "internal/platform/container/container.go")

	out, err := wiring.Inject(read(t, path), wiring.Injection{
		Path:   path,
		Marker: fieldsMarker,
		Code:   "// Order handles HTTP requests for the order feature.\nOrder *order.Handler",
	})
	require.NoError(t, err)
	requireParses(t, out)

	text := string(out)
	assert.Contains(t, text, "Order *order.Handler")
	assert.Less(t, lineIndex(text, "Order *order.Handler"), lineIndex(text, fieldsMarker))
	assert.Contains(t, text, "Logger *slog.Logger", "the anchor field is preserved")
	assert.Contains(t, text, "// Logger is the structured logger shared by the application.")
}

func TestInjectIntoEmptyStruct(t *testing.T) {
	t.Parallel()

	src := `package container

type Container struct {
	` + fieldsMarker + `
}
`
	out, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "container.go",
		Marker: fieldsMarker,
		Code:   "Order *order.Handler",
	})
	require.NoError(t, err)
	requireParses(t, out)
	assert.Contains(t, string(out), "Order *order.Handler")
	assert.Contains(t, string(out), fieldsMarker)
}

func TestInjectIntoEmptyFunctionBody(t *testing.T) {
	t.Parallel()

	src := `package router

func register() {
	` + routerMarker + `
}
`
	out, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   "setup()",
	})
	require.NoError(t, err)
	requireParses(t, out)
	assert.Contains(t, string(out), "setup()")
	assert.Contains(t, string(out), routerMarker)
}

func TestInjectTwiceKeepsBothFeatures(t *testing.T) {
	t.Parallel()

	root := project(t)
	path := filepath.Join(root, "internal/platform/router/router.go")
	src := read(t, path)

	first, err := wiring.Inject(src, wiring.Injection{
		Path:   path,
		Marker: routerMarker,
		Code:   "c.Order.RegisterRoutes(mux)",
	})
	require.NoError(t, err)

	second, err := wiring.Inject(first, wiring.Injection{
		Path:   path,
		Marker: routerMarker,
		Code:   "c.Product.RegisterRoutes(mux)",
	})
	require.NoError(t, err)
	requireParses(t, second)

	text := string(second)
	assert.Contains(t, text, "c.Order.RegisterRoutes(mux)")
	assert.Contains(t, text, "c.Product.RegisterRoutes(mux)")
	assert.Equal(t, 1, wiring.CountMarker(second, routerMarker), "the marker itself must not be duplicated")
}

func TestInjectAddsImportBlockWhenThereIsNone(t *testing.T) {
	t.Parallel()

	src := "package router\n\nfunc New() {\n\t" + routerMarker + "\n}\n"
	out, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:    "router.go",
		Marker:  routerMarker,
		Code:    `logger := strings.TrimSpace(" x ")`,
		Imports: []string{"strings"},
	})
	require.NoError(t, err)
	requireParses(t, out)

	assert.Contains(t, string(out), `"strings"`)
	assert.Contains(t, string(out), "logger := strings.TrimSpace")
}

func TestInjectGrowsASingleImport(t *testing.T) {
	t.Parallel()

	src := `package router

import "net/http"

func New() *http.ServeMux {
	` + routerMarker + `
	_ = http.MethodGet
	return nil
}
`
	out, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:    "router.go",
		Marker:  routerMarker,
		Code:    `logger := strings.TrimSpace(" x ")`,
		Imports: []string{"strings"},
	})
	require.NoError(t, err)
	requireParses(t, out)

	assert.Contains(t, string(out), `"net/http"`)
	assert.Contains(t, string(out), `"strings"`)
}

func TestInjectKeepsExistingImport(t *testing.T) {
	t.Parallel()

	root := project(t)
	path := filepath.Join(root, "internal/platform/container/container.go")

	out, err := wiring.Inject(read(t, path), wiring.Injection{
		Path:    path,
		Marker:  wiringMarker,
		Code:    "logger := slog.Default()",
		Imports: []string{"log/slog"},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(string(out), `"log/slog"`), "an existing import must not be duplicated")
}

func TestInjectTopLevelDeclarations(t *testing.T) {
	t.Parallel()

	src := "package router\n\n" + routerMarker + "\nfunc New() {\n}\n"
	out, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   "// Version is the build version.\nconst Version = \"dev\"",
	})
	require.NoError(t, err)
	requireParses(t, out)

	assert.Contains(t, string(out), "const Version =")
	assert.Contains(t, string(out), routerMarker)
}

func TestInjectRefusesAnUnsupportedPosition(t *testing.T) {
	t.Parallel()

	src := `package router

var names = []string{
	` + routerMarker + `
	"a",
}
`
	_, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   `"b",`,
	})
	require.Error(t, err)

	var position *wiring.MarkerPositionError
	require.ErrorAs(t, err, &position)
	assert.Contains(t, err.Error(), "not in a supported position")
}

func TestInjectMissingMarker(t *testing.T) {
	t.Parallel()

	src := "package router\n\nfunc New() {}\n"
	_, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   "setup()",
	})

	var notFound *wiring.MarkerNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, routerMarker, notFound.Marker)
	assert.Contains(t, err.Error(), "router.go")
}

func TestInjectDuplicateMarker(t *testing.T) {
	t.Parallel()

	src := "package router\n\nfunc New() {\n\t" + routerMarker + "\n\t" + routerMarker + "\n}\n"
	_, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   "setup()",
	})

	var dup *wiring.DuplicateMarkerError
	require.ErrorAs(t, err, &dup)
	assert.Equal(t, 2, dup.Count)
}

func TestInjectEmptyMarker(t *testing.T) {
	t.Parallel()

	_, err := wiring.Inject([]byte("package x\n"), wiring.Injection{Path: "x.go", Code: "setup()"})
	assert.ErrorIs(t, err, wiring.ErrEmptyMarker)
}

func TestInjectInvalidCode(t *testing.T) {
	t.Parallel()

	src := "package router\n\nfunc New() {\n\t" + routerMarker + "\n}\n"
	_, err := wiring.Inject([]byte(src), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   "this is not go code",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid statement code")
}

func TestInjectBrokenSource(t *testing.T) {
	t.Parallel()

	_, err := wiring.Inject([]byte("package router\nfunc New( {\n"), wiring.Injection{
		Path:   "router.go",
		Marker: routerMarker,
		Code:   "setup()",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestContainsMarker(t *testing.T) {
	t.Parallel()

	assert.True(t, wiring.ContainsMarker([]byte(routerTemplate), routerMarker))
	assert.False(t, wiring.ContainsMarker([]byte("package router\n"), routerMarker))
	assert.Equal(t, 0, wiring.CountMarker(nil, ""))
}

func TestRemoveInjectedCode(t *testing.T) {
	t.Parallel()

	root := project(t)
	path := filepath.Join(root, "internal/platform/container/container.go")

	code := strings.Join([]string{
		"orderRepo := order.NewInMemoryRepository()",
		"orderService := order.NewService(orderRepo)",
		"c.Order = order.NewHandler(orderService)",
	}, "\n")

	wired, err := wiring.Inject(read(t, path), wiring.Injection{
		Path:    path,
		Marker:  wiringMarker,
		Code:    code,
		Imports: []string{featureImport},
	})
	require.NoError(t, err)

	out, err := wiring.Remove(wired, wiring.Removal{
		Path: path,
		Lines: []string{
			"orderRepo := order.NewInMemoryRepository()",
			"orderService := order.NewService(orderRepo)",
			"c.Order = order.NewHandler(orderService)",
		},
	})
	require.NoError(t, err)
	requireParses(t, out)

	text := string(out)
	assert.NotContains(t, text, "orderRepo")
	assert.NotContains(t, text, featureImport, "unused imports are dropped")
	assert.Contains(t, text, wiringMarker)
	assert.Contains(t, text, "Logger: slog.Default()")
}

func TestRemoveMissingLine(t *testing.T) {
	t.Parallel()

	src := "package router\n\nfunc New() {}\n"
	_, err := wiring.Remove([]byte(src), wiring.Removal{Path: "router.go", Lines: []string{"setup()"}})

	var missing *wiring.LineNotFoundError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "setup()", missing.Line)
}

func TestRemoveRejectsUnparseableSource(t *testing.T) {
	t.Parallel()

	_, err := wiring.Remove([]byte("package router\nfunc New( {\n"), wiring.Removal{
		Path:  "router.go",
		Lines: []string{"setup()"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestFormatSourceFallsBackToGofmt(t *testing.T) {
	t.Parallel()

	out, err := wiring.FormatSource("", []byte("package x\n\nfunc F()   {   }\n"))
	require.NoError(t, err)
	assert.Equal(t, "package x\n\nfunc F() {}\n", string(out))

	_, err = wiring.FormatSource("", []byte("this is not go\n"))
	require.Error(t, err)
}

func TestRemoveAmbiguousLine(t *testing.T) {
	t.Parallel()

	src := "package router\n\nfunc New() {\n\tsetup()\n\tsetup()\n}\n"
	_, err := wiring.Remove([]byte(src), wiring.Removal{Path: "router.go", Lines: []string{"setup()"}})

	var ambiguous *wiring.AmbiguousLineError
	require.ErrorAs(t, err, &ambiguous)
	assert.Equal(t, 2, ambiguous.Count)
}
