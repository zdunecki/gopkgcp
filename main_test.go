package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{"skip testdata", "testdata", true},
		{"skip vendor", "vendor", true},
		{"skip .git", ".git", true},
		{"skip _test suffix", "foo_test", true},
		{"allow regular dir", "utils", false},
		{"allow src", "src", false},
		{"allow internal", "internal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipDir(tt.dirName)
			if result != tt.expected {
				t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.dirName, result, tt.expected)
			}
		})
	}
}

func TestShouldCopyFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		expected bool
	}{
		{"copy .go file", "main.go", true},
		{"copy nested .go file", "utils.go", true},
		{"skip test file", "main_test.go", false},
		{"skip other test file", "utils_test.go", false},
		{"copy go.mod", "go.mod", true},
		{"copy go.sum", "go.sum", true},
		{"copy LICENSE", "LICENSE", true},
		{"copy README.md", "README.md", true},
		{"skip .txt file", "notes.txt", false},
		{"skip .json file", "config.json", false},
		{"skip .yaml file", "config.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldCopyFile(tt.fileName)
			if result != tt.expected {
				t.Errorf("shouldCopyFile(%q) = %v, want %v", tt.fileName, result, tt.expected)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gopkgcp-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.go")
	content := []byte("package main\n\nfunc main() {}\n")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest.go")
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination exists and has same content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("destination content = %q, want %q", string(dstContent), string(content))
	}
}

func TestCopyDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gopkgcp-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory structure
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	// Create .go file (should be copied)
	goFile := filepath.Join(srcDir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write go file: %v", err)
	}

	// Create test file (should be skipped)
	testFile := filepath.Join(srcDir, "main_test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create .txt file (should be skipped)
	txtFile := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dst")
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify .go file was copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); err != nil {
		t.Errorf("main.go should have been copied: %v", err)
	}

	// Verify test file was NOT copied
	if _, err := os.Stat(filepath.Join(dstDir, "main_test.go")); !os.IsNotExist(err) {
		t.Errorf("main_test.go should NOT have been copied")
	}

	// Verify .txt file was NOT copied
	if _, err := os.Stat(filepath.Join(dstDir, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("notes.txt should NOT have been copied")
	}
}

func TestReplaceModuleInFiles(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gopkgcp-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldModule := "github.com/openai/openai-go/v3"
	newModule := "github.com/myorg/myproject"

	// Create go.mod file
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := "module github.com/openai/openai-go/v3\n\ngo 1.21\n"
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create .go file with imports
	goFilePath := filepath.Join(tmpDir, "main.go")
	goFileContent := `package main

import (
	"fmt"

	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/internal/util"
)

func main() {
	fmt.Println("hello")
}
`
	if err := os.WriteFile(goFilePath, []byte(goFileContent), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Create a .txt file (should NOT be modified)
	txtPath := filepath.Join(tmpDir, "notes.txt")
	txtContent := "Contains github.com/openai/openai-go/v3 reference"
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		t.Fatalf("failed to write notes.txt: %v", err)
	}

	// Run replacement
	if err := replaceModuleInFiles(tmpDir, oldModule, newModule); err != nil {
		t.Fatalf("replaceModuleInFiles failed: %v", err)
	}

	// Verify go.mod was updated
	goModResult, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	expectedGoMod := "module github.com/myorg/myproject\n\ngo 1.21\n"
	if string(goModResult) != expectedGoMod {
		t.Errorf("go.mod = %q, want %q", string(goModResult), expectedGoMod)
	}

	// Verify .go file was updated
	goFileResult, err := os.ReadFile(goFilePath)
	if err != nil {
		t.Fatalf("failed to read main.go: %v", err)
	}
	expectedGoFile := `package main

import (
	"fmt"

	"github.com/myorg/myproject/responses"
	"github.com/myorg/myproject/internal/util"
)

func main() {
	fmt.Println("hello")
}
`
	if string(goFileResult) != expectedGoFile {
		t.Errorf("main.go = %q, want %q", string(goFileResult), expectedGoFile)
	}

	// Verify .txt file was NOT modified
	txtResult, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("failed to read notes.txt: %v", err)
	}
	if string(txtResult) != txtContent {
		t.Errorf("notes.txt should not have been modified, got %q, want %q", string(txtResult), txtContent)
	}
}
