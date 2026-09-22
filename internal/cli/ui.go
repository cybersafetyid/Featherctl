package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/cybersafetyid/featherctl/internal/generator"
)

// Symbols used in feather's output.
const (
	symbolOK   = "✔"
	symbolFail = "✘"
	symbolWarn = "⚠"
)

// styles holds the lipgloss styles used by the CLI.
//
// When the destination is not a terminal every style is the zero value, and
// lipgloss renders plain text — no escapes leak into pipes or test buffers.
type styles struct {
	success lipgloss.Style
	failure lipgloss.Style
	warning lipgloss.Style
	muted   lipgloss.Style
	bold    lipgloss.Style
}

// newStyles returns the styles to use when writing to w.
func newStyles(w io.Writer) styles {
	if !isTerminal(w) {
		return styles{}
	}

	return styles{
		success: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		failure: lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		warning: lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		bold:    lipgloss.NewStyle().Bold(true),
	}
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// isTerminalReader reports whether r is an interactive terminal.
func isTerminalReader(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(file)
}

// printer renders the human readable output of every command.
type printer struct {
	out    io.Writer
	errOut io.Writer
	styles styles
}

// newPrinter returns a printer that writes normal output to out and problems
// to errOut.
func newPrinter(out, errOut io.Writer) *printer {
	return &printer{out: out, errOut: errOut, styles: newStyles(out)}
}

// PrintError renders a command failure the way every feather command does.
func PrintError(w io.Writer, err error) {
	newPrinter(w, w).error(err)
}

// report renders a generator run, one line per change. A dry run reads the
// same but in the future tense, so the output of a dry run and a real run stay
// easy to compare.
func (p *printer) report(report *generator.Report) {
	for _, change := range report.Changes {
		verb := reportVerb(change.Action, report.DryRun)
		if verb == "" {
			continue
		}
		p.line(p.styles.success, symbolOK, verb+" "+change.Rel)
	}
}

// reportVerb renders the action of a change, in the tense the run calls for.
func reportVerb(action generator.Action, dryRun bool) string {
	verbs := map[generator.Action][2]string{
		generator.ActionCreated:        {"Created", "Would create"},
		generator.ActionOverwritten:    {"Overwrote", "Would overwrite"},
		generator.ActionModified:       {"Modified", "Would modify"},
		generator.ActionRemoved:        {"Removed", "Would remove"},
		generator.ActionWiredRoutes:    {"Wired routes into", "Would wire routes into"},
		generator.ActionWiredContainer: {"Wired container into", "Would wire container into"},
	}

	pair, ok := verbs[action]
	if !ok {
		return ""
	}
	if dryRun {
		return pair[1]
	}
	return pair[0]
}

// check renders one doctor check.
func (p *printer) check(check generator.Check) {
	style, symbol := p.styles.success, symbolOK
	switch check.Status {
	case generator.StatusWarn:
		style, symbol = p.styles.warning, symbolWarn
	case generator.StatusFail:
		style, symbol = p.styles.failure, symbolFail
	}

	p.line(style, symbol, check.Name+": "+check.Message)
	if check.Detail == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(check.Detail, "\n"), "\n") {
		fmt.Fprintf(p.out, "    %s\n", p.styles.muted.Render(line))
	}
}

// summary writes the closing line of a successful command, preceded by a blank
// line.
func (p *printer) summary(format string, args ...any) {
	fmt.Fprintf(p.out, "\n"+format+"\n", args...)
}

// hint writes an indented follow-up line.
func (p *printer) hint(format string, args ...any) {
	fmt.Fprintf(p.out, "  "+format+"\n", args...)
}

// dryRun warns that a run changed nothing on disk.
func (p *printer) dryRun() {
	p.line(p.styles.warning, symbolWarn, "Dry run: nothing was written")
}

// line writes "<symbol> <message>".
func (p *printer) line(style lipgloss.Style, symbol, message string) {
	fmt.Fprintf(p.out, "%s %s\n", style.Render(symbol), message)
}

// error writes a failure to the error stream.
func (p *printer) error(err error) {
	fmt.Fprintf(p.errOut, "%s %s\n", p.styles.failure.Render(symbolFail), err)
}
