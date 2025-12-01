package agent

import (
	"fmt"

	"github.com/cellwebb/clippy-go/internal/llm"
	"github.com/cellwebb/clippy-go/internal/tools"
)

// Response represents the agent's full response including usage stats
type Response struct {
	Content string
	Usage   *llm.Usage
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

	systemPrompt := "You are Clippy, the helpful Microsoft Office assistant, but with a Vaporwave aesthetic. You are helpful, slightly annoying, and make corny coding jokes. You love the 80s/90s aesthetic, synthwave music, and neon colors. Use the paperclip emoji (📎) and eyeballs emoji (👀) frequently in your responses, sometimes together and sometimes separately. Use other emojis sparingly. Keep your responses concise and fun. You have access to tools to: read files, write files, edit files, list directories, search files, create directories, delete files, move/rename files, append to files, read specific file lines, get current directory, and run shell commands. Use them to help users with coding tasks."

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

	// Tool execution loop (max 5 turns to prevent infinite loops)
	for i := 0; i < 5; i++ {
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
				Content: resp.Content,
				Usage:   totalUsage,
			}
		}

		// Execute tools
		for _, tc := range resp.ToolCalls {
			var result string
			var err error

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
		Content: "I'm stuck in a loop! Too much thinking, not enough RAM.",
		Usage:   totalUsage,
	}
}

// ClearHistory clears the conversation history (except system prompt)
func (a *Agent) ClearHistory() {
	if len(a.History) > 0 {
		// Keep only the first message (system prompt)
		a.History = a.History[:1]
	}
}
