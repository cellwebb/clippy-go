package ui

import (
	"fmt"
	"strings"

	"github.com/cellwebb/clippy-go/internal/agent"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Vaporwave colors
const (
	ColorCyan   = "#00FFFF"
	ColorPink   = "#FF71CE"
	ColorPurple = "#B967FF"
	ColorYellow = "#FFF68F"
	ColorBg     = "#1a1a2e"
	ColorBorder = "#B967FF"
)

var (
	stylePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPink)).Bold(true)
	styleUser   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
	styleClippy = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorYellow))
	styleStatus = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple)).Italic(true)
	styleBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(0, 1)
	styleHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPink)).
			Bold(true).
			Align(lipgloss.Center).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(0, 1)
	styleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPurple)).
			Faint(true)
)

type keyMap struct {
	Quit key.Binding
	Help key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "esc"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
}

type model struct {
	agent      *agent.Agent
	viewport   viewport.Model
	help       help.Model
	messages   []string
	input      string
	quitting   bool
	spinner    spinner.Model
	loading    bool
	width      int
	height     int
	ready      bool
	toolStatus string
	showHelp   bool
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
		help:     help.New(),
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
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 5
		footerHeight := 3
		statusHeight := 1
		inputHeight := 1

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight-statusHeight-inputHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight - statusHeight - inputHeight
		}

		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			if !m.loading {
				m.quitting = true
				return m, tea.Quit
			}
		case "?":
			m.showHelp = !m.showHelp
		case "enter":
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
				m.viewport.SetContent("")
				return m, nil
			}
			if m.input == "/help" {
				m.showHelp = !m.showHelp
				m.input = ""
				return m, nil
			}

			// Add user message
			m.messages = append(m.messages, styleUser.Render("You: ")+m.input)
			m.updateViewport()

			cmd := m.getAgentResponse(m.input)
			m.input = ""
			m.loading = true
			m.toolStatus = "Thinking..."
			return m, tea.Batch(m.spinner.Tick, cmd)

		case "backspace", "delete":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			// Regular typing
			m.input += msg.String()
		}

	case responseMsg:
		m.loading = false
		m.toolStatus = ""
		m.messages = append(m.messages, styleClippy.Render("Clippy: ")+string(msg))
		m.updateViewport()
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Handle viewport scrolling
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) updateViewport() {
	content := strings.Join(m.messages, "\n\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m model) View() string {
	if m.quitting {
		return stylePrompt.Render("See you in the V O I D! ✨") + "\n"
	}

	if !m.ready {
		return "Initializing..."
	}

	// Header
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
	headerContent := lipgloss.JoinVertical(lipgloss.Center,
		styleClippy.Render(clippyArt),
		stylePrompt.Render("V A P O R W A V E   C L I P P Y"),
	)
	header := styleHeader.Width(m.width - 2).Render(headerContent)

	// Viewport
	viewportStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Width(m.width - 2).
		Height(m.viewport.Height)

	viewportContent := viewportStyle.Render(m.viewport.View())

	// Status bar
	var statusText string
	if m.loading {
		statusText = fmt.Sprintf("%s %s", m.spinner.View(), m.toolStatus)
	} else {
		statusText = fmt.Sprintf("Ready | Messages: %d | Tools: 12 available", len(m.messages)/2)
	}
	statusBar := styleStatus.Width(m.width - 2).Render(statusText)

	// Input area
	var inputArea string
	if m.loading {
		inputArea = "⏳ Working..."
	} else {
		inputArea = stylePrompt.Render("> ") + m.input + "█"
	}
	inputBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Width(m.width-2).
		Padding(0, 1).
		Render(inputArea)

	// Footer
	var footerText string
	if m.showHelp {
		footerText = "Commands: /quit /exit /clear /new /reset /help | Keys: ? (help) ctrl+c (quit)"
	} else {
		footerText = "/quit /clear /help | ? for more help | ctrl+c to exit"
	}
	footer := styleFooter.Width(m.width - 2).Render(footerText)

	// Combine all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		viewportContent,
		statusBar,
		inputBox,
		footer,
	)
}
