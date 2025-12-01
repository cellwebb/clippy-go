package ui

import (
	"fmt"
	"strings"

	"github.com/cellwebb/clippy-go/internal/agent"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
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
	agent         *agent.Agent
	viewport      viewport.Model
	help          help.Model
	messages      []string
	textInput     textinput.Model
	quitting      bool
	spinner       spinner.Model
	loading       bool
	width         int
	height        int
	ready         bool
	toolStatus    string
	showHelp      bool
	lastUsage     *agent.Response
	totalTokens   int
	suggestions   []string
	suggestionIdx int
}

var availableCommands = []string{
	"/quit", "/exit", "/clear", "/new", "/reset", "/help",
}

func InitialModel(agt *agent.Agent) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPink))

	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80
	ti.Prompt = stylePrompt.Render("> ")
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))

	return model{
		agent:     agt,
		messages:  []string{},
		textInput: ti,
		spinner:   s,
		help:      help.New(),
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

type responseMsg struct {
	content string
	usage   *agent.Response
}

func (m model) getAgentResponse(input string) tea.Cmd {
	return func() tea.Msg {
		resp := m.agent.GetResponse(input)
		return responseMsg{
			content: resp.Content,
			usage:   &resp,
		}
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
		case "up":
			if len(m.suggestions) > 0 {
				m.suggestionIdx--
				if m.suggestionIdx < 0 {
					m.suggestionIdx = len(m.suggestions) - 1
				}
				return m, nil
			}
			// Forward to textinput if no suggestions
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		case "down":
			if len(m.suggestions) > 0 {
				m.suggestionIdx++
				if m.suggestionIdx >= len(m.suggestions) {
					m.suggestionIdx = 0
				}
				return m, nil
			}
			// Forward to textinput if no suggestions
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		case "tab":
			if len(m.suggestions) > 0 {
				m.textInput.SetValue(m.suggestions[m.suggestionIdx])
				m.suggestions = nil
				m.suggestionIdx = 0
				m.updateSuggestions()
				return m, nil
			}

		case "enter":
			input := m.textInput.Value()
			if len(m.suggestions) > 0 {
				m.textInput.SetValue(m.suggestions[m.suggestionIdx])
				m.suggestions = nil
				m.suggestionIdx = 0
				m.updateSuggestions()
				return m, nil
			}

			if input == "" {
				return m, nil
			}

			// Handle slash commands
			if input == "/quit" || input == "/exit" {
				m.quitting = true
				return m, tea.Quit
			}
			if input == "/clear" || input == "/new" || input == "/reset" {
				m.messages = []string{}
				m.textInput.SetValue("")
				m.viewport.SetContent("")
				return m, nil
			}
			if input == "/help" {
				m.showHelp = !m.showHelp
				m.textInput.SetValue("")
				return m, nil
			}

			// Add user message
			m.messages = append(m.messages, styleUser.Render("You: ")+input)
			m.updateViewport()

			cmd := m.getAgentResponse(input)
			m.textInput.SetValue("")
			m.loading = true
			m.toolStatus = "Thinking..."
			return m, tea.Batch(m.spinner.Tick, cmd)

		default:
			// Forward to textinput
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			m.updateSuggestions()
			return m, cmd
		}

	case responseMsg:
		m.loading = false
		m.toolStatus = ""
		m.messages = append(m.messages, styleClippy.Render("Clippy: ")+msg.content)
		if msg.usage != nil && msg.usage.Usage != nil {
			m.totalTokens += msg.usage.Usage.TotalTokens
			m.lastUsage = msg.usage
		}
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

func (m *model) updateSuggestions() {
	input := m.textInput.Value()
	if !strings.HasPrefix(input, "/") {
		m.suggestions = nil
		m.suggestionIdx = 0
		return
	}

	m.suggestions = []string{}
	for _, cmd := range availableCommands {
		if strings.HasPrefix(cmd, input) {
			m.suggestions = append(m.suggestions, cmd)
		}
	}
	m.suggestionIdx = 0
}

func (m *model) updateViewport() {
	width := m.width - 6 // Account for borders and padding
	if width < 0 {
		width = 0
	}

	var wrappedMessages []string
	for _, msg := range m.messages {
		wrappedMessages = append(wrappedMessages, wordwrap.String(msg, width))
	}

	content := strings.Join(wrappedMessages, "\n\n")
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
		usageInfo := ""
		if m.totalTokens > 0 {
			usageInfo = fmt.Sprintf(" | Tokens: %d", m.totalTokens)
		}
		statusText = fmt.Sprintf("Ready | Messages: %d%s", len(m.messages)/2, usageInfo)
	}
	statusBar := styleStatus.Width(m.width - 2).Render(statusText)

	// Input area
	var inputArea string
	if m.loading {
		inputArea = "⏳ Working..."
	} else {
		inputArea = m.textInput.View()
	}
	inputBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Width(m.width-2).
		Padding(0, 1).
		Render(inputArea)

	// Suggestions
	var suggestionsView string
	if len(m.suggestions) > 0 {
		var s []string
		for i, sug := range m.suggestions {
			if i == m.suggestionIdx {
				s = append(s, stylePrompt.Render("> "+sug))
			} else {
				s = append(s, "  "+sug)
			}
		}
		suggestionsView = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Width(m.width - 2).
			Render(strings.Join(s, "\n"))
	}

	// Footer
	var footerText string
	if m.showHelp {
		footerText = "Commands: /quit /exit /clear /new /reset /help | Keys: ? (help) ctrl+c (quit)"
	} else {
		footerText = "/quit /clear /help | ? for more help | ctrl+c to exit"
	}
	footer := styleFooter.Width(m.width - 2).Render(footerText)

	// Combine all sections
	if suggestionsView != "" {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			viewportContent,
			statusBar,
			suggestionsView,
			inputBox,
			footer,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		viewportContent,
		statusBar,
		inputBox,
		footer,
	)
}
