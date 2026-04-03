package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"breachline/tui"
	"breachline/tui/adapters"

	tea "github.com/charmbracelet/bubbletea"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	// CLI flags
	queryFlag := flag.String("q", "", "Apply initial query")
	flag.StringVar(queryFlag, "query", "", "Apply initial query")

	tzFlag := flag.String("t", "", "Set display timezone")
	flag.StringVar(tzFlag, "timezone", "", "Set display timezone")

	ingestTZFlag := flag.String("ingest-tz", "", "Set ingest timezone")

	noHeader := flag.Bool("no-header", false, "Treat first row as data (CSV)")

	jpathFlag := flag.String("j", "", "JSONPath expression (JSON files)")
	flag.StringVar(jpathFlag, "jpath", "", "JSONPath expression (JSON files)")

	workspaceFlag := flag.String("w", "", "Open workspace file")
	flag.StringVar(workspaceFlag, "workspace", "", "Open workspace file")

	licenseFlag := flag.String("l", "", "Import license file")
	flag.StringVar(licenseFlag, "license", "", "Import license file")

	noSort := flag.Bool("no-sort", false, "Disable time-based sorting")
	sortDesc := flag.Bool("sort-desc", false, "Sort descending (default: ascending)")
	showVersion := flag.Bool("version", false, "Show version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: breachline-cli [flags] [file...]\n\n")
		fmt.Fprintf(os.Stderr, "BreachLine CLI - Terminal UI for log file analysis\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  breachline-cli access.log\n")
		fmt.Fprintf(os.Stderr, "  breachline-cli -q \"status=500 | after 1h\" nginx.csv\n")
		fmt.Fprintf(os.Stderr, "  breachline-cli -j '$.events[*]' data.json\n")
		fmt.Fprintf(os.Stderr, "  breachline-cli -w investigation.breachline\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("breachline-cli %s\n", version)
		os.Exit(0)
	}

	files := flag.Args()
	if len(files) == 0 && *workspaceFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: no files specified")
		fmt.Fprintln(os.Stderr, "Run 'breachline-cli --help' for usage")
		os.Exit(1)
	}

	// Set up logging to stderr so it doesn't interfere with the TUI
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[breachline] ")

	// Initialise backend services
	appInstance, settingsService, licenseService, workspaceService := adapters.InitBackend()
	backend := adapters.NewBackend(appInstance, settingsService, licenseService)
	backend.SetWorkspace(workspaceService)

	opts := tui.CLIOptions{
		Files:       files,
		Query:       *queryFlag,
		Timezone:    *tzFlag,
		IngestTZ:    *ingestTZFlag,
		NoHeader:    *noHeader,
		JPath:       *jpathFlag,
		Workspace:   *workspaceFlag,
		LicensePath: *licenseFlag,
		NoSort:      *noSort,
		SortDesc:    *sortDesc,
		Version:     version,
	}

	// Create and run the Bubble Tea program
	model := tui.NewApp(backend, opts)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	backend.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
