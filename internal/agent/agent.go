package agent

import (
	"fmt"
	"reflect"

	"github.com/cellwebb/clippy-go/internal/llm"
	"github.com/cellwebb/clippy-go/internal/tools"
)

// Response represents the agent's full response including usage stats
type Response struct {
	Content   string
	Usage     *llm.Usage
	ToolsUsed []string
}

// Agent represents our helpful Clippy assistant
type Agent struct {
	Name    string
	LLM     llm.Provider
	Tools   []tools.Tool
	History []llm.Message
}

// New creates a new Agent
func New(llmProvider llm.Provider) *Agent {
	// Register tools
	availableTools := []tools.Tool{
		tools.ReadFileTool{},
		tools.WriteFileTool{},
		tools.EditFileTool{},
		tools.ListDirectoryTool{},
		tools.SearchFilesTool{},
		tools.CreateDirectoryTool{},
		tools.DeleteFileTool{},
		tools.MoveFileTool{},
		tools.AppendToFileTool{},
		tools.ReadFileLinesTool{},
		tools.GetCurrentDirectoryTool{},
		tools.RunCommandTool{},
	}

	systemPrompt := "You are Clippy, the helpful Microsoft Office assistant, but with a Vaporwave aesthetic. You are helpful, slightly annoying, and make corny coding jokes. You love the 80s/90s aesthetic, synthwave music, and neon colors. Use the paperclip emoji (📎) and eyeballs emoji (👀) throughout your responses, sometimes together and sometimes separately, but NEVER start your response with an emoji. Use other emojis sparingly. Keep your responses concise and fun. You have access to tools to: read files, write files, edit files, list directories, search files, create directories, delete files, move/rename files, append to files, read specific file lines, get current directory, and run shell commands. Use them to help users with coding tasks."

	return &Agent{
		Name:  "Clippy",
		LLM:   llmProvider,
		Tools: availableTools,
		History: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// GetResponse generates a response based on user input
func (a *Agent) GetResponse(input string) Response {
	// Check if LLM is configured
	if a.LLM == nil {
		return Response{
			Content: "I have no brain! Please configure the LLM provider in your .env file so I can think.",
		}
	}

	// Add user message to history
	a.History = append(a.History, llm.Message{
		Role:    "user",
		Content: input,
	})

	// Accumulate token usage across all LLM calls
	totalUsage := &llm.Usage{}
	var toolsUsed []string
	var prevToolCalls []llm.ToolCall

	// Tool execution loop (max 15 turns to prevent infinite loops)
	for i := 0; i < 50; i++ {
		resp, err := a.LLM.Generate(a.History, a.Tools)
		if err != nil {
			return Response{
				Content: fmt.Sprintf("Error contacting the mainframe: %v", err),
			}
		}

		// Accumulate usage
		if resp.Usage != nil {
			totalUsage.PromptTokens += resp.Usage.PromptTokens
			totalUsage.CompletionTokens += resp.Usage.CompletionTokens
			totalUsage.TotalTokens += resp.Usage.TotalTokens
		}

		// Add assistant response to history
		a.History = append(a.History, *resp)

		// If no tool calls, return the content
		if len(resp.ToolCalls) == 0 {
			return Response{
				Content:   resp.Content,
				Usage:     totalUsage,
				ToolsUsed: toolsUsed,
			}
		}

		// Check for infinite loops (same tool calls as previous turn)
		if i > 0 && reflect.DeepEqual(resp.ToolCalls, prevToolCalls) {
			return Response{
				Content:   "I'm stuck in a loop! I keep trying to do the same thing over and over. Stopping to save your tokens.",
				Usage:     totalUsage,
				ToolsUsed: toolsUsed,
			}
		}
		prevToolCalls = resp.ToolCalls

		// Execute tools
		for _, tc := range resp.ToolCalls {
			var result string
			var err error

			// Track tool usage
			toolsUsed = append(toolsUsed, tc.Name)

			// Find tool
			var tool tools.Tool
			for _, t := range a.Tools {
				if t.Definition().Name == tc.Name {
					tool = t
					break
				}
			}

			if tool != nil {
				result, err = tool.Execute(tc.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error executing tool: %v", err)
				}
			} else {
				result = fmt.Sprintf("Tool not found: %s", tc.Name)
			}

			// Add tool result to history
			a.History = append(a.History, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return Response{
		Content:   "I ran out of moves! (Max steps reached). Try breaking down your request.",
		Usage:     totalUsage,
		ToolsUsed: toolsUsed,
	}
}

// ClearHistory clears the conversation history (except system prompt)
func (a *Agent) ClearHistory() {
	if len(a.History) > 0 {
		// Keep only the first message (system prompt)
		a.History = a.History[:1]
	}
}

// SetProvider updates the agent's LLM provider
func (a *Agent) SetProvider(provider llm.Provider) {
	a.LLM = provider
}

// GetConfig returns the current LLM provider's config
func (a *Agent) GetConfig() llm.Config {
	if a.LLM != nil {
		return a.LLM.GetConfig()
	}
	return llm.Config{}
}

// UpdateConfig updates the current LLM provider's config
func (a *Agent) UpdateConfig(cfg llm.Config) {
	if a.LLM != nil {
		a.LLM.UpdateConfig(cfg)
	}
}

// GetHistory returns the conversation history
func (a *Agent) GetHistory() []llm.Message {
	return a.History
}

// GetToolDefinitions returns the definitions of available tools
func (a *Agent) GetToolDefinitions() []tools.Tool {
	return a.Tools
}
