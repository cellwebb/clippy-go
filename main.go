package main

import (
	"fmt"
	"os"

	"github.com/cellwebb/clippy-go/internal/agent"
	"github.com/cellwebb/clippy-go/internal/llm"
	"github.com/cellwebb/clippy-go/internal/ui"
	"github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	godotenv.Load()

	// Load config
	cfg := llm.LoadConfigFromEnv()

	// Initialize LLM provider
	var llmProvider llm.Provider
	var err error
	if cfg.Provider != "" {
		llmProvider, err = llm.NewProvider(cfg)
		if err != nil {
			fmt.Printf("Error initializing LLM provider: %v\n", err)
			os.Exit(1)
		}
	}

	// Initialize agent
	agt := agent.New(llmProvider)

	// Start UI
	p := tea.NewProgram(ui.InitialModel(agt))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
