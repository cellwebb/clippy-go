package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"

	// Test WriteFileTool
	writeTool := WriteFileTool{}
	_, err := writeTool.Execute(map[string]interface{}{
		"path":    filePath,
		"content": content,
	})
	if err != nil {
		t.Fatalf("WriteFileTool failed: %v", err)
	}

	// Test ReadFileTool
	readTool := ReadFileTool{}
	readContent, err := readTool.Execute(map[string]interface{}{
		"path": filePath,
	})
	if err != nil {
		t.Fatalf("ReadFileTool failed: %v", err)
	}

	if readContent != content {
		t.Errorf("Expected content %q, got %q", content, readContent)
	}
}

func TestEditFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "edit_test.txt")
	initialContent := "Hello, World!"

	// Create initial file
	err := os.WriteFile(filePath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test EditFileTool
	editTool := EditFileTool{}
	_, err = editTool.Execute(map[string]interface{}{
		"path":        filePath,
		"target":      "World",
		"replacement": "Clippy",
	})
	if err != nil {
		t.Fatalf("EditFileTool failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "Hello, Clippy!"
	if string(content) != expected {
		t.Errorf("Expected content %q, got %q", expected, string(content))
	}
}

func TestListDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	// Test ListDirectoryTool
	listTool := ListDirectoryTool{}
	output, err := listTool.Execute(map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("ListDirectoryTool failed: %v", err)
	}

	if !strings.Contains(output, "file1.txt") {
		t.Error("Output missing file1.txt")
	}
	if !strings.Contains(output, "file2.txt") {
		t.Error("Output missing file2.txt")
	}
	if !strings.Contains(output, "subdir") {
		t.Error("Output missing subdir")
	}
}

func TestCreateAndDeleteDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "newdir")

	// Test CreateDirectoryTool
	createTool := CreateDirectoryTool{}
	_, err := createTool.Execute(map[string]interface{}{
		"path": newDir,
	})
	if err != nil {
		t.Fatalf("CreateDirectoryTool failed: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil || !info.IsDir() {
		t.Error("Directory was not created")
	}
}

func TestAppendToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "append.txt")

	// Create initial file
	os.WriteFile(filePath, []byte("Line 1\n"), 0644)

	// Test AppendToFileTool
	appendTool := AppendToFileTool{}
	_, err := appendTool.Execute(map[string]interface{}{
		"path":    filePath,
		"content": "Line 2",
	})
	if err != nil {
		t.Fatalf("AppendToFileTool failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "Line 1\nLine 2"
	if string(content) != expected {
		t.Errorf("Expected content %q, got %q", expected, string(content))
	}
}
