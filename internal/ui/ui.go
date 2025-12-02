package ui

import (
	"fmt"
	"strings"

	"github.com/cellwebb/clippy-go/internal/agent"
	"github.com/cellwebb/clippy-go/internal/llm"
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
	Quit     key.Binding
	Help     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
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
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "scroll up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "scroll down"),
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
	"/quit", "/exit", "/clear", "/new", "/reset", "/help", "/provider", "/model", "/status",
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
		case "pgup":
			// Scroll viewport up by a page
			scrollAmount := m.viewport.Height / 2
			if scrollAmount < 1 {
				scrollAmount = 1
			}
			m.viewport.LineUp(scrollAmount)
			return m, nil
		case "pgdown":
			// Scroll viewport down by a page
			scrollAmount := m.viewport.Height / 2
			if scrollAmount < 1 {
				scrollAmount = 1
			}
			m.viewport.LineDown(scrollAmount)
			return m, nil

		case "enter":
			input := m.textInput.Value()

			// If suggestions are showing but input already matches exactly, execute it
			if len(m.suggestions) > 0 {
				// Check if input is already an exact match
				isExactMatch := false
				for _, cmd := range availableCommands {
					if input == cmd {
						isExactMatch = true
						break
					}
				}

				// If not an exact match, select the suggestion
				if !isExactMatch {
					m.textInput.SetValue(m.suggestions[m.suggestionIdx])
					m.suggestions = nil
					m.suggestionIdx = 0
					m.updateSuggestions()
					return m, nil
				}

				// If exact match, clear suggestions and continue to execute
				m.suggestions = nil
				m.suggestionIdx = 0
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
				m.agent.ClearHistory()
				return m, nil
			}

			if strings.HasPrefix(input, "/provider") {
				parts := strings.Fields(input)
				if len(parts) > 1 {
					provider := parts[1]
					// Update provider
					cfg := m.agent.GetConfig()
					cfg.Provider = provider
					m.agent.UpdateConfig(cfg)
					m.messages = append(m.messages, styleStatus.Render(fmt.Sprintf("[⚙️] Provider set to: %s", provider)))
				} else {
					// List providers
					m.messages = append(m.messages, styleStatus.Render("[⚙️] Available providers: openai, anthropic"))
				}
				m.textInput.SetValue("")
				m.updateViewport()
				return m, nil
			}

			if strings.HasPrefix(input, "/model") {
				parts := strings.Fields(input)
				if len(parts) > 1 {
					modelName := parts[1]
					// Update model
					cfg := m.agent.GetConfig()
					cfg.Model = modelName
					m.agent.UpdateConfig(cfg)
					m.messages = append(m.messages, styleStatus.Render(fmt.Sprintf("[⚙️] Model set to: %s", modelName)))
					m.textInput.SetValue("")
					m.updateViewport()
					return m, nil
				} else {
					// Fetch models
					m.loading = true
					m.toolStatus = "Fetching models..."
					return m, tea.Batch(m.spinner.Tick, fetchModelsCmd())
				}
			}
			if input == "/help" {
				m.showHelp = !m.showHelp
				m.textInput.SetValue("")
				return m, nil
			}
			
			if input == "/status" {
				// Get config status
				cfg := m.agent.GetConfig()
				statusMsg := fmt.Sprintf("\n%s[⚙️] CONFIG STATUS%s\n", styleHeader.Render(""), styleHeader.Render(""))
				statusMsg += fmt.Sprintf("%sProvider: %s\n", styleStatus.Render("  "), styleClippy.Render(cfg.Provider))
				statusMsg += fmt.Sprintf("%sModel: %s\n", styleStatus.Render("  "), styleClippy.Render(cfg.Model))
				if cfg.BaseURL != "" {
					statusMsg += fmt.Sprintf("%sBase URL: %s\n", styleStatus.Render("  "), styleClippy.Render(cfg.BaseURL))
				} else {
					if cfg.Provider == "openai" {
						statusMsg += fmt.Sprintf("%sBase URL: %s\n", styleStatus.Render("  "), styleClippy.Render("https://api.openai.com/v1"))
					} else if cfg.Provider == "anthropic" {
						statusMsg += fmt.Sprintf("%sBase URL: %s\n", styleStatus.Render("  "), styleClippy.Render("https://api.anthropic.com/v1"))
					} else {
						statusMsg += fmt.Sprintf("%sBase URL: %s\n", styleStatus.Render("  "), styleClippy.Render("default"))
					}
				}
				if cfg.APIKey != "" {
					statusMsg += fmt.Sprintf("%sAPI Key: %s (%s...%s)\n", styleStatus.Render("  "), styleClippy.Render("***configured***"), cfg.APIKey[:4], cfg.APIKey[len(cfg.APIKey)-4:])
				} else {
					statusMsg += fmt.Sprintf("%sAPI Key: %s\n", styleStatus.Render("  "), styleClippy.Render("not set"))
				}
				
				// Message breakdown
				statusMsg += fmt.Sprintf("\n%s[📊] MESSAGE BREAKDOWN%s\n", styleHeader.Render(""), styleHeader.Render(""))
				
				systemCount := 0
				userCount := 0
				assistantCount := 0
				toolCount := 0
				systemTokens := 0
				userTokens := 0
				assistantTokens := 0
				toolTokens := 0
				
				for _, msg := range m.agent.GetHistory() {
					switch msg.Role {
					case "system":
						systemCount++
						if msg.Usage != nil {
							systemTokens += msg.Usage.TotalTokens
						}
					case "user":
						userCount++
						if msg.Usage != nil {
							userTokens += msg.Usage.TotalTokens
						}
					case "assistant":
						assistantCount++
						if msg.Usage != nil {
							assistantTokens += msg.Usage.TotalTokens
						}
					case "tool":
						toolCount++
						if msg.Usage != nil {
							toolTokens += msg.Usage.TotalTokens
						}
					}
				}
				
				statusMsg += fmt.Sprintf("%sSystem messages: %s%d%s (%s%d%s tokens)\n", 
					styleStatus.Render("  "), stylePrompt.Render(""), systemCount, styleStatus.Render(""), 
					styleHeader.Render(""), systemTokens, styleStatus.Render(""))
				statusMsg += fmt.Sprintf("%sUser messages: %s%d%s (%s%d%s tokens)\n", 
					styleStatus.Render("  "), styleUser.Render(""), userCount, styleStatus.Render(""), 
					styleHeader.Render(""), userTokens, styleStatus.Render(""))
				statusMsg += fmt.Sprintf("%sAssistant messages: %s%d%s (%s%d%s tokens)\n", 
					styleStatus.Render("  "), styleClippy.Render(""), assistantCount, styleStatus.Render(""), 
					styleHeader.Render(""), assistantTokens, styleStatus.Render(""))
				statusMsg += fmt.Sprintf("%sTool calls/responses: %s%d%s (%s%d%s tokens)\n", 
					styleStatus.Render("  "), stylePrompt.Render(""), toolCount, styleStatus.Render(""), 
					styleHeader.Render(""), toolTokens, styleStatus.Render(""))
				statusMsg += fmt.Sprintf("%sTotal messages: %s%d%s\n", styleStatus.Render("  "), styleHeader.Render(""), len(m.agent.GetHistory()), styleStatus.Render(""))
				
				// Token usage
				statusMsg += fmt.Sprintf("\n%s[🪙] TOKEN USAGE%s\n", styleHeader.Render(""), styleHeader.Render(""))
				if m.totalTokens > 0 {
					if m.lastUsage != nil && m.lastUsage.Usage != nil {
						statusMsg += fmt.Sprintf("%sLast call - Prompt: %s%d%s | Completion: %s%d%s | Total: %s%d%s\n", 
							styleStatus.Render("  "), 
							stylePrompt.Render(""), m.lastUsage.Usage.PromptTokens, styleStatus.Render(""),
							styleClippy.Render(""), m.lastUsage.Usage.CompletionTokens, styleStatus.Render(""),
							styleHeader.Render(""), m.lastUsage.Usage.TotalTokens, styleStatus.Render(""))
					}
					statusMsg += fmt.Sprintf("%sSession total: %s%d%s tokens\n", 
						styleStatus.Render("  "), 
						styleHeader.Render(""), m.totalTokens, styleStatus.Render(""))
					
					// Calculate average tokens per message
					if userCount > 0 {
						avgTokens := m.totalTokens / userCount
						statusMsg += fmt.Sprintf("%sAverage per exchange: %s%d%s tokens\n", 
							styleStatus.Render("  "), styleHeader.Render(""), avgTokens, styleStatus.Render(""))
					}
					
					// estimated cost (rough calculations)
					var estimatedCost string
					if cfg.Provider == "openai" {
						// Rough estimates for GPT-4
						cost := float64(m.totalTokens) * 0.00003 // $0.03 per 1K tokens
						estimatedCost = fmt.Sprintf("$%.4f", cost)
					} else if cfg.Provider == "anthropic" {
						// Rough estimates for Claude
						cost := float64(m.totalTokens) * 0.00003 // $0.03 per 1K tokens
						estimatedCost = fmt.Sprintf("$%.4f", cost)
					} else {
						estimatedCost = "unknown"
					}
					statusMsg += fmt.Sprintf("%sEstimated cost: %s%s%s\n", 
						styleStatus.Render("  "), styleHeader.Render(""), estimatedCost, styleStatus.Render(""))
				} else {
					statusMsg += fmt.Sprintf("%sNo tokens used yet in this session\n", styleStatus.Render("  "))
				}
				
				// Last tools used
				if m.lastUsage != nil && len(m.lastUsage.ToolsUsed) > 0 {
					statusMsg += fmt.Sprintf("\n%s[🔧] RECENT TOOLS%s\n", styleHeader.Render(""), styleHeader.Render(""))
					statusMsg += fmt.Sprintf("%sLast used: %s\n", styleStatus.Render("  "), styleClippy.Render(strings.Join(m.lastUsage.ToolsUsed, ", ")))
					
					// Count tool usage frequency
					toolUsage := make(map[string]int)
					for _, msg := range m.agent.GetHistory() {
						if msg.Role == "tool" {
							// Extract tool name from content if possible, or track by tool call
							for _, tc := range msg.ToolCalls {
								toolUsage[tc.Name]++
							}
						}
					}
					
					if len(toolUsage) > 0 {
						statusMsg += fmt.Sprintf("%sUsage frequency: ", styleStatus.Render("  "))
						var toolFreq []string
						for tool, count := range toolUsage {
							toolFreq = append(toolFreq, fmt.Sprintf("%s%s:%d", styleClippy.Render(tool), styleStatus.Render(""), count))
						}
						statusMsg += strings.Join(toolFreq, " | ") + "\n"
					}
				}
				
				// Available tools count
				statusMsg += fmt.Sprintf("\n%s[🛠️] TOOLS AVAILABLE%s\n", styleHeader.Render(""), styleHeader.Render(""))
				toolDefs := m.agent.GetToolDefinitions()
				statusMsg += fmt.Sprintf("%sTotal tools: %s%d%s\n", styleStatus.Render("  "), stylePrompt.Render(""), len(toolDefs), styleStatus.Render(""))
				
				// List available tools
				statusMsg += fmt.Sprintf("%sAvailable: ", styleStatus.Render("  "))
				var toolNames []string
				for _, tool := range toolDefs {
					toolNames = append(toolNames, tool.Definition().Name)
				}
				statusMsg += styleClippy.Render(strings.Join(toolNames, ", ")) + "\n"
				
				// Session stats
				statusMsg += fmt.Sprintf("\n%s[📈] SESSION STATS%s\n", styleHeader.Render(""), styleHeader.Render(""))
				statusMsg += fmt.Sprintf("%sSession duration: %sActive%s\n", styleStatus.Render("  "), styleClippy.Render(""), styleStatus.Render(""))
				if m.agent.LLM != nil {
					statusMsg += fmt.Sprintf("%sLLM Status: %sConnected%s\n", styleStatus.Render("  "), styleClippy.Render(""), styleStatus.Render(""))
				} else {
					statusMsg += fmt.Sprintf("%sLLM Status: %sNot configured%s\n", styleStatus.Render("  "), stylePrompt.Render(""), styleStatus.Render(""))
				}
				
				m.messages = append(m.messages, statusMsg)
				m.textInput.SetValue("")
				m.updateViewport()
				return m, nil
			}

			// Add user message
			m.messages = append(m.messages, styleUser.Render("[You] ")+input)
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

	case modelsMsg:
		m.loading = false
		m.toolStatus = ""
		if msg.err != nil {
			m.messages = append(m.messages, styleStatus.Render(fmt.Sprintf("[❌] Error fetching models: %v", msg.err)))
		} else {
			m.messages = append(m.messages, styleStatus.Render(fmt.Sprintf("[⚙️] Available models: %s", strings.Join(msg.models, ", "))))
		}
		m.updateViewport()
		return m, nil

	case responseMsg:
		m.loading = false
		m.toolStatus = ""

		// Show which tools were used
		if msg.usage != nil && len(msg.usage.ToolsUsed) > 0 {
			toolMsg := styleStatus.Render(fmt.Sprintf("[🔧] Tools used: %s", strings.Join(msg.usage.ToolsUsed, ", ")))
			m.messages = append(m.messages, toolMsg)
		}

		// Strip any leading emojis and whitespace from the content
		content := msg.content
		for len(content) > 0 {
			r, size := []rune(content)[0], len([]rune(content)[0:1])
			// Check if it's an emoji or whitespace
			if r > 127 || r == ' ' || r == '\t' || r == '\n' {
				content = content[size:]
			} else {
				break
			}
		}

		m.messages = append(m.messages, styleClippy.Render("[📎] ")+content)
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

	// Handle viewport scrolling (including mouse wheel!)
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
		statusText = fmt.Sprintf("Ready | Messages: %d%s | Use mouse wheel to scroll through history", len(m.messages)/2, usageInfo)
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
		footerText = "Commands: /quit /exit /clear /new /reset /help /status | Keys: ? (help) ctrl+c (quit) pgup/pgdown (scroll) | Mouse wheel scrolls chat history"
	} else {
		footerText = "/quit /clear /help /status | ? for more help | pgup/pgdown or mouse wheel to scroll | ctrl+c to exit"
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

type modelsMsg struct {
	models []string
	err    error
}

func fetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		models, err := llm.FetchModels()
		return modelsMsg{models: models, err: err}
	}
}