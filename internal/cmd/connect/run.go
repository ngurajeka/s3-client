package connect

import (
	"context"
	"flag"
	"fmt"
	"os"

	"s3-client/internal/shared/config"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/logging"
	tea "github.com/charmbracelet/bubbletea"
)

type noopLogger struct{}

func (l noopLogger) Logf(classification logging.Classification, format string, v ...interface{}) {}

func newFlagSet() *flag.FlagSet {
	return flag.NewFlagSet("connect", flag.ContinueOnError)
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintln(os.Stderr, "Usage: s3-client connect [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fs.PrintDefaults()
}

func Run(args []string) int {
	fs := newFlagSet()

	opts := &config.Options{}
	config.AddFlags(fs, opts)

	fs.Usage = func() {
		printUsage(fs)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	awsCfg, err := config.Load(context.Background(), *opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load AWS config: %v\n", err)
		return 1
	}

	_ = noopLogger{}

	client := s3.NewFromConfig(awsCfg)

	m := initialModel(client)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	m.program = p

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		return 1
	}

	return 0
}
