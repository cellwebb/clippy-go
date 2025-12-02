package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ToolDefinition describes a tool to the LLM
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema
}

// Tool represents an executable tool
type Tool interface {
	Definition() ToolDefinition
	Execute(args map[string]interface{}) (string, error)
}

// ReadFileTool reads a file from disk
type ReadFileTool struct{}

func (t ReadFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "read_file",
		Description: "Read the contents of a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t ReadFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return string(content), nil
}

// WriteFileTool writes content to a file
type WriteFileTool struct{}

func (t WriteFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write_file",
		Description: "Write content to a file (overwrites existing content)",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (t WriteFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'content' argument")
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote to %s", path), nil
}

// RunCommandTool executes a shell command
type RunCommandTool struct{}

func (t RunCommandTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "run_command",
		Description: "Execute a shell command",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to execute",
				},
			},
			"required": []string{"command"},
		},
	}
}

func (t RunCommandTool) Execute(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'command' argument")
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Command failed: %v\nOutput:\n%s", err, string(output)), nil
	}

	return string(output), nil
}

// EditFileTool edits a file by replacing a target string with replacement string
type EditFileTool struct{}

func (t EditFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "edit_file",
		Description: "Edit a file by replacing a specific target string with a replacement string",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to edit",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "The exact string to replace",
				},
				"replacement": map[string]interface{}{
					"type":        "string",
					"description": "The new string to replace the target with",
				},
			},
			"required": []string{"path", "target", "replacement"},
		},
	}
}

func (t EditFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}
	target, ok := args["target"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'target' argument")
	}
	replacement, ok := args["replacement"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'replacement' argument")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, target) {
		return "", fmt.Errorf("target string not found in file")
	}

	newText := strings.Replace(text, target, replacement, 1)

	err = os.WriteFile(path, []byte(newText), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully edited %s", path), nil
}

// ListDirectoryTool lists files and directories in a path
type ListDirectoryTool struct{}

func (t ListDirectoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_directory",
		Description: "List all files and subdirectories in a directory",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory path to list (use '.' for current directory)",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t ListDirectoryTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of %s:\n", path))
	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("  [DIR]  %s\n", entry.Name()))
		} else {
			info, _ := entry.Info()
			result.WriteString(fmt.Sprintf("  [FILE] %s (%d bytes)\n", entry.Name(), info.Size()))
		}
	}
	return result.String(), nil
}

// SearchFilesTool searches for text patterns in files
type SearchFilesTool struct{}

func (t SearchFilesTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_files",
		Description: "Search for a text pattern in files within a directory (recursive)",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory to search in",
				},
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The text pattern to search for",
				},
			},
			"required": []string{"path", "pattern"},
		},
	}
}

func (t SearchFilesTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'pattern' argument")
	}

	cmd := exec.Command("grep", "-r", "-n", pattern, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// grep returns exit code 1 if no matches found
		if len(output) == 0 {
			return "No matches found", nil
		}
	}

	return string(output), nil
}

// CreateDirectoryTool creates a new directory
type CreateDirectoryTool struct{}

func (t CreateDirectoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_directory",
		Description: "Create a new directory (including parent directories if needed)",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory path to create",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t CreateDirectoryTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	return fmt.Sprintf("Successfully created directory %s", path), nil
}

// DeleteFileTool deletes a file
type DeleteFileTool struct{}

func (t DeleteFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_file",
		Description: "Delete a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The file path to delete",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t DeleteFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	err := os.Remove(path)
	if err != nil {
		return "", fmt.Errorf("failed to delete file: %v", err)
	}

	return fmt.Sprintf("Successfully deleted %s", path), nil
}

// MoveFileTool moves or renames a file
type MoveFileTool struct{}

func (t MoveFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "move_file",
		Description: "Move or rename a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source": map[string]interface{}{
					"type":        "string",
					"description": "The source file path",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "The destination file path",
				},
			},
			"required": []string{"source", "destination"},
		},
	}
}

func (t MoveFileTool) Execute(args map[string]interface{}) (string, error) {
	source, ok := args["source"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'source' argument")
	}
	destination, ok := args["destination"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'destination' argument")
	}

	err := os.Rename(source, destination)
	if err != nil {
		return "", fmt.Errorf("failed to move file: %v", err)
	}

	return fmt.Sprintf("Successfully moved %s to %s", source, destination), nil
}

// AppendToFileTool appends content to a file
type AppendToFileTool struct{}

func (t AppendToFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "append_to_file",
		Description: "Append content to the end of a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The file path to append to",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to append",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (t AppendToFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'content' argument")
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to append to file: %v", err)
	}

	return fmt.Sprintf("Successfully appended to %s", path), nil
}

// ReadFileLinesTools reads specific line ranges from a file
type ReadFileLinesTool struct{}

func (t ReadFileLinesTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "read_file_lines",
		Description: "Read specific line ranges from a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The file path to read",
				},
				"start_line": map[string]interface{}{
					"type":        "number",
					"description": "Starting line number (1-indexed)",
				},
				"end_line": map[string]interface{}{
					"type":        "number",
					"description": "Ending line number (1-indexed)",
				},
			},
			"required": []string{"path", "start_line", "end_line"},
		},
	}
}

func (t ReadFileLinesTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}
	startLineFloat, ok := args["start_line"].(float64)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'start_line' argument")
	}
	endLineFloat, ok := args["end_line"].(float64)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'end_line' argument")
	}

	startLine := int(startLineFloat)
	endLine := int(endLineFloat)

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	if startLine < 1 || startLine > len(lines) {
		return "", fmt.Errorf("start_line out of range")
	}
	if endLine < startLine || endLine > len(lines) {
		return "", fmt.Errorf("end_line out of range")
	}

	selectedLines := lines[startLine-1 : endLine]
	return strings.Join(selectedLines, "\n"), nil
}

// GetCurrentDirectoryTool gets the current working directory
type GetCurrentDirectoryTool struct{}

func (t GetCurrentDirectoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_current_directory",
		Description: "Get the current working directory",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

func (t GetCurrentDirectoryTool) Execute(args map[string]interface{}) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}
	return dir, nil
}

// FormatToolExecution creates a human-readable description of a tool execution
func FormatToolExecution(toolName string, args map[string]interface{}) string {
	switch toolName {
	case "read_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📖 Reading file: %s", path)
		}
	case "write_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("✍️  Writing file: %s", path)
		}
	case "edit_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("✏️  Editing file: %s", path)
		}
	case "list_directory":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📁 Listing directory: %s", path)
		}
	case "search_files":
		if path, ok := args["path"].(string); ok {
			if pattern, ok := args["pattern"].(string); ok {
				return fmt.Sprintf("🔍 Searching in %s for: %s", path, pattern)
			}
			return fmt.Sprintf("🔍 Searching in: %s", path)
		}
	case "create_directory":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📂 Creating directory: %s", path)
		}
	case "delete_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("🗑️  Deleting file: %s", path)
		}
	case "move_file":
		if source, ok := args["source"].(string); ok {
			if dest, ok := args["destination"].(string); ok {
				return fmt.Sprintf("📦 Moving %s → %s", source, dest)
			}
			return fmt.Sprintf("📦 Moving: %s", source)
		}
	case "append_to_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("➕ Appending to: %s", path)
		}
	case "read_file_lines":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📖 Reading lines from: %s", path)
		}
	case "run_command":
		if command, ok := args["command"].(string); ok {
			return fmt.Sprintf("⚡ Running: %s", command)
		}
	case "get_current_directory":
		return "📍 Getting current directory"
	}
	
	// Fallback format
	return fmt.Sprintf("🔧 Executing: %s", toolName)
}
