package agent

import (
	"fmt"

	"github.com/cellwebb/clippy-go/internal/llm"
	"github.com/cellwebb/clippy-go/internal/tools"
)

// Agent represents our helpful Clippy assistant
type Agent struct {
	Name  string
	LLM   llm.Provider
	Tools []tools.Tool
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

	return &Agent{
		Name:  "Clippy",
		LLM:   llmProvider,
		Tools: availableTools,
	}
}

// GetResponse generates a response based on user input
func (a *Agent) GetResponse(input string) string {
	// Check if LLM is configured
	if a.LLM == nil {
		return "I have no brain! Please configure the LLM provider in your .env file so I can think."
	}

	systemPrompt := "You are Clippy, the helpful Microsoft Office assistant, but with a Vaporwave aesthetic. You are helpful, slightly annoying, and make corny coding jokes. You love the 80s/90s aesthetic, synthwave music, and neon colors. Keep your responses concise and fun. You have access to tools to: read files, write files, edit files, list directories, search files, create directories, delete files, move/rename files, append to files, read specific file lines, get current directory, and run shell commands. Use them to help users with coding tasks."

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: input},
	}

	// Tool execution loop (max 5 turns to prevent infinite loops)
	for i := 0; i < 5; i++ {
		resp, err := a.LLM.Generate(messages, a.Tools)
		if err != nil {
			return fmt.Sprintf("Error contacting the mainframe: %v", err)
		}

		messages = append(messages, *resp)

		// If no tool calls, return the content
		if len(resp.ToolCalls) == 0 {
			return resp.Content
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

			// Add tool result to messages
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return "I'm stuck in a loop! Too much thinking, not enough RAM."
}
