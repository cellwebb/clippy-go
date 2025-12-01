package ui

import (
	"fmt"

	"github.com/cellwebb/clippy-go/internal/agent"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Vaporwave colors
const (
	ColorCyan   = "#00FFFF"
	ColorPink   = "#FF71CE"
	ColorPurple = "#B967FF"
	ColorYellow = "#FFF68F" // For Clippy's body
)

var (
	stylePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPink)).Bold(true)
	styleUser   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
	styleClippy = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorYellow))
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorPurple)).
			Padding(1, 2)
)

type model struct {
	agent    *agent.Agent
	messages []string
	input    string
	quitting bool
	spinner  spinner.Model
	loading  bool
}

func InitialModel(agt *agent.Agent) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPink))

	return model{
		agent:    agt,
		messages: []string{},
		input:    "",
		spinner:  s,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

type responseMsg string

func (m model) getAgentResponse(input string) tea.Cmd {
	return func() tea.Msg {
		return responseMsg(m.agent.GetResponse(input))
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.input == "" {
				return m, nil
			}

			// Handle slash commands
			if m.input == "/quit" || m.input == "/exit" {
				m.quitting = true
				return m, tea.Quit
			}
			if m.input == "/clear" || m.input == "/new" || m.input == "/reset" {
				m.messages = []string{}
				m.input = ""
				return m, nil
			}

			// Add user message
			m.messages = append(m.messages, styleUser.Render("You: ")+m.input)

			cmd := m.getAgentResponse(m.input)
			m.input = ""
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, cmd)
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case tea.KeySpace:
			m.input += " "
		case tea.KeyRunes:
			m.input += string(msg.Runes)
		}

	case responseMsg:
		m.loading = false
		m.messages = append(m.messages, styleClippy.Render("Clippy: ")+string(msg))
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	// ASCII Art Clippy
	clippyArt := `
   __
  /  \
  |  |
  @  @
  |  |
  || |
  || |
  |__|
  `

	// Header
	header := styleBorder.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			styleClippy.Render(clippyArt),
			stylePrompt.Render("V A P O R W A V E   C L I P P Y"),
		),
	)

	// Chat history
	var chatHistory string
	start := 0
	if len(m.messages) > 10 {
		start = len(m.messages) - 10
	}
	for _, msg := range m.messages[start:] {
		chatHistory += msg + "\n"
	}

	// Input area
	var inputArea string
	if m.loading {
		inputArea = fmt.Sprintf("\n%s %s", m.spinner.View(), "Thinking...")
	} else {
		inputArea = fmt.Sprintf("\n%s %s", stylePrompt.Render(">"), m.input)
	}

	return fmt.Sprintf("%s\n\n%s%s\n", header, chatHistory, inputArea)
}
