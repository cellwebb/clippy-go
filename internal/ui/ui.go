package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cellwebb/clippy-go/internal/agent"
	"github.com/cellwebb/clippy-go/internal/llm"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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



type model struct {
	agent         *agent.Agent
	viewport      viewport.Model
	help          help.Model
	messages      []string
	textArea      textarea.Model
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

	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.CharLimit = 2000
	ta.SetWidth(80)
	ta.SetHeight(1)
	ta.Prompt = "" // Remove prompt from textarea, will add it manually
	ta.ShowLineNumbers = false
	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
	ta.FocusedStyle.Base = cyanStyle
	ta.FocusedStyle.Text = cyanStyle
	ta.FocusedStyle.Placeholder = cyanStyle.Faint(true)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Base = cyanStyle
	ta.BlurredStyle.Text = cyanStyle
	ta.BlurredStyle.Placeholder = cyanStyle.Faint(true)
	ta.KeyMap.InsertNewline.SetEnabled(true) // Allow newlines, Ctrl+Enter to send

	return model{
		agent:    agt,
		messages: []string{},
		textArea: ta,
		spinner:  s,
		help:     help.New(),
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
		inputHeight := m.textArea.Height()

		m.width = msg.Width
		m.height = msg.Height
		m.textArea.SetWidth(msg.Width - 4) // Adjust textarea width to window
		m.resizeTextarea() // Recalculate height after width change
		inputHeight = m.textArea.Height() // Get updated height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight-statusHeight-inputHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight - statusHeight - inputHeight
		}

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
			// Forward to textarea if no suggestions
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			return m, cmd
		case "down":
			if len(m.suggestions) > 0 {
				m.suggestionIdx++
				if m.suggestionIdx >= len(m.suggestions) {
					m.suggestionIdx = 0
				}
				return m, nil
			}
			// Forward to textarea if no suggestions
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			return m, cmd
		case "shift+enter":
			// Handle newline in textarea
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			// Auto-resize textarea based on content
			m.resizeTextarea()
			return m, cmd
		case "tab":
			if len(m.suggestions) > 0 {
				m.textArea.SetValue(m.suggestions[m.suggestionIdx])
				m.suggestions = nil
				m.suggestionIdx = 0
				m.updateSuggestions()
				m.resizeTextarea()
				return m, nil
			}
		case "pgup":
			// Scroll viewport up by a page
			scrollAmount := m.viewport.Height / 2
			if scrollAmount < 1 {
				scrollAmount = 1
			}
			m.viewport.ScrollUp(scrollAmount)
			return m, nil
		case "pgdown":
			// Scroll viewport down by a page
			scrollAmount := m.viewport.Height / 2
			if scrollAmount < 1 {
				scrollAmount = 1
			}
			m.viewport.ScrollDown(scrollAmount)
			return m, nil

		case "ctrl+enter":
			// Continue with the original enter logic for sending messages
		case "enter":
			input := m.textArea.Value()

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
					m.textArea.SetValue(m.suggestions[m.suggestionIdx])
					m.suggestions = nil
					m.suggestionIdx = 0
					m.updateSuggestions()
					m.resizeTextarea()
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
				m.textArea.SetValue("")
				m.textArea.SetHeight(1)
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
				m.textArea.SetValue("")
				m.textArea.SetHeight(1)
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
					m.textArea.SetValue("")
					m.textArea.SetHeight(1)
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
				m.textArea.SetValue("")
				m.textArea.SetHeight(1)
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
					switch cfg.Provider {
					case "openai":
						statusMsg += fmt.Sprintf("%sBase URL: %s\n", styleStatus.Render("  "), styleClippy.Render("https://api.openai.com/v1"))
					case "anthropic":
						statusMsg += fmt.Sprintf("%sBase URL: %s\n", styleStatus.Render("  "), styleClippy.Render("https://api.anthropic.com/v1"))
					default:
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
					switch cfg.Provider {
					case "openai":
						// Rough estimates for GPT-4
						cost := float64(m.totalTokens) * 0.00003 // $0.03 per 1K tokens
						estimatedCost = fmt.Sprintf("$%.4f", cost)
					case "anthropic":
						// Rough estimates for Claude
						cost := float64(m.totalTokens) * 0.00003 // $0.03 per 1K tokens
						estimatedCost = fmt.Sprintf("$%.4f", cost)
					default:
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
				m.textArea.SetValue("")
				m.textArea.SetHeight(1)
				m.updateViewport()
				return m, nil
			}

			// Add user message
			m.messages = append(m.messages, styleUser.Render("[You] ")+input)
			m.updateViewport()

			cmd := m.getAgentResponse(input)
			m.textArea.SetValue("")
			m.textArea.SetHeight(1)
			m.loading = true
			m.toolStatus = "Thinking..."
			return m, tea.Batch(m.spinner.Tick, cmd)

		default:
			// Forward to textarea
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			// Auto-resize textarea based on content
			m.resizeTextarea()
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
	input := m.textArea.Value()
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

// wrapText wraps text to the specified width, preserving newlines
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	// Split by existing newlines
	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		if len(line) == 0 {
			wrappedLines = append(wrappedLines, "")
			continue
		}

		// Wrap each line
		for len(line) > width {
			// Find the last space before the width limit
			breakPoint := width
			for ; breakPoint > 0 && !unicode.IsSpace(rune(line[breakPoint])); breakPoint-- {
			}

			if breakPoint == 0 {
				// No spaces found, break at width
				breakPoint = width
			}

			wrappedLines = append(wrappedLines, strings.TrimSpace(line[:breakPoint]))
			line = strings.TrimSpace(line[breakPoint:])
		}

		if len(line) > 0 {
			wrappedLines = append(wrappedLines, line)
		}
	}

	return strings.Join(wrappedLines, "\n")
}

// resizeTextarea automatically adjusts the textarea height based on content
func (m *model) resizeTextarea() {
	content := m.textArea.Value()
	if content == "" {
		m.textArea.SetHeight(1)
		return
	}

	// Apply word wrapping and update content
	textareaWidth := m.textArea.Width()
	if textareaWidth <= 0 {
		textareaWidth = 80
	}
	wrappedContent := wrapText(content, textareaWidth)

	// Only update if the content has actually changed and no suggestions are showing
	// (to avoid interfering with tab completion)
	if wrappedContent != content && len(m.suggestions) == 0 {
		// For now, just update without trying to preserve cursor position
		// This is a limitation of the textarea component
		m.textArea.SetValue(wrappedContent)
	}

	// Calculate the height based on wrapped lines
	lines := strings.Count(wrappedContent, "\n") + 1

	// Set min height to 1, max height to 10 for reasonable UX
	maxHeight := 10
	if m.height > 20 {
		// Don't let textarea take more than 1/2 of available height
		maxHeight = (m.height - 10) / 2
	}
	if maxHeight < 3 {
		maxHeight = 3
	}
	if lines < 1 {
		lines = 1
	}
	if lines > maxHeight {
		lines = maxHeight
	}
	m.textArea.SetHeight(lines)
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
	var inputBox string
	if m.loading {
		inputArea := stylePrompt.Render("> ") + "⏳ Working..."
		inputBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Width(m.width - 2).
			Padding(0, 1).
			Render(inputArea)
	} else {
		// Add prompt manually only for the first line
		textareaContent := m.textArea.View()
		lines := strings.Split(textareaContent, "\n")
		if len(lines) > 0 {
			// Add prompt only to the first line
			lines[0] = stylePrompt.Render("> ") + styleUser.Render(lines[0])
			// Apply cyan style to all other lines
			for i := 1; i < len(lines); i++ {
				if lines[i] != "" {
					lines[i] = styleUser.Render(lines[i])
				}
			}
			textareaContent = strings.Join(lines, "\n")
		}
		inputBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Width(m.width - 2).
			Padding(0, 1).
			Render(textareaContent)
	}

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
		footerText = "Commands: /quit /exit /clear /new /reset /help /status | Keys: ? (help) ctrl+c (quit) pgup/pgdown (scroll) Ctrl+Enter (send) | Mouse wheel scrolls chat history"
	} else {
		footerText = "/quit /clear /help /status | ? for more help | pgup/pgdown or mouse wheel to scroll | Ctrl+Enter to send | ctrl+c to exit"
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