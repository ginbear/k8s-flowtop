package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ginbear/k8s-flowtop/internal/k8s"
	"github.com/ginbear/k8s-flowtop/internal/tui"
)

var (
	version   = "dev"
	namespace = flag.String("n", "", "Kubernetes namespace (empty for all namespaces)")
	showVer   = flag.Bool("v", false, "Show version")
)

func main() {
	flag.Parse()

	if *showVer {
		fmt.Printf("k8s-flowtop %s\n", version)
		os.Exit(0)
	}

	client, err := k8s.NewClient(*namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	model := tui.NewModel(client)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
