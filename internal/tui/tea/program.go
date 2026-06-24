package tea

import (
	"context"
	"io"

	bubbletea "github.com/charmbracelet/bubbletea"
)

// RunProgram launches a Bubble Tea program for the given model using the
// supplied terminal streams. It is the only place in this package that directly
// constructs a Bubble Tea program, keeping all direct
// github.com/charmbracelet/bubbletea imports inside internal/tui/tea.
func RunProgram(ctx context.Context, in io.Reader, out io.Writer, model bubbletea.Model) error {
	program := bubbletea.NewProgram(
		model,
		bubbletea.WithContext(ctx),
		bubbletea.WithInput(in),
		bubbletea.WithOutput(out),
	)
	_, err := program.Run()
	return err
}
