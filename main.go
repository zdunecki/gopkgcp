package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Parse flags
	pkg := flag.String("pkg", "", "Package path to extract (e.g., ./responses)")
	output := flag.String("o", "", "Output directory")
	moduleOnly := flag.Bool("module-only", true, "Only extract packages from the same module (exclude external deps)")
	modName := flag.String("mod", "", "Override module name in extracted files (e.g., github.com/myorg/myproject)")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	if *pkg == "" || *output == "" {
		fmt.Fprintf(os.Stderr, "Usage: gopkgcp -pkg <package> -o <dir>\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  gopkgcp -pkg ./responses -o ./extracted\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get current module path
	modulePath, moduleDir, err := getModuleInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting module info: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're running this from a Go module directory\n")
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Module: %s\n", modulePath)
		fmt.Printf("Module dir: %s\n", moduleDir)
	}

	// Run goda to get dependencies
	selector := ":all"
	if *moduleOnly {
		selector = ":mod"
	}

	godaExpr := *pkg + selector
	if *verbose {
		fmt.Printf("Running: goda list %s\n", godaExpr)
	}

	deps, err := runGoda(godaExpr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running goda: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure goda is installed: go install github.com/loov/goda@latest\n")
		os.Exit(1)
	}

	if len(deps) == 0 {
		fmt.Fprintf(os.Stderr, "No packages found for %s\n", *pkg)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Found %d packages to extract:\n", len(deps))
		for _, d := range deps {
			fmt.Printf("  - %s\n", d)
		}
	}

	// Create output directory
	if err := os.MkdirAll(*output, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Filter and copy packages
	copied := 0
	for _, dep := range deps {
		// Skip external dependencies if module-only
		if !strings.HasPrefix(dep, modulePath) {
			if *verbose {
				fmt.Printf("Skipping external: %s\n", dep)
			}
			continue
		}

		// Convert package path to relative directory
		relPath := strings.TrimPrefix(dep, modulePath)
		relPath = strings.TrimPrefix(relPath, "/")

		srcDir := filepath.Join(moduleDir, relPath)
		dstDir := filepath.Join(*output, relPath)

		if *verbose {
			fmt.Printf("Copying: %s -> %s\n", srcDir, dstDir)
		}

		if err := copyDir(srcDir, dstDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying %s: %v\n", relPath, err)
			continue
		}
		copied++
	}

	fmt.Printf("✓ Extracted %d packages to %s\n", copied, *output)

	// Copy go.mod and go.sum if they exist
	for _, f := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(moduleDir, f)
		dst := filepath.Join(*output, f)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not copy %s: %v\n", f, err)
			} else if *verbose {
				fmt.Printf("Copied %s\n", f)
			}
		}
	}

	// Replace module name if -mod flag is provided
	if *modName != "" {
		if *verbose {
			fmt.Printf("Replacing module %s with %s\n", modulePath, *modName)
		}
		if err := replaceModuleInFiles(*output, modulePath, *modName); err != nil {
			fmt.Fprintf(os.Stderr, "Error replacing module name: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Replaced module name with %s\n", *modName)
	}

	// Run go mod tidy in output directory
	if *verbose {
		fmt.Printf("Running go mod tidy in %s\n", *output)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = *output
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr

	if err := tidyCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: go mod tidy failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "You may need to run it manually: cd %s && go mod tidy\n", *output)
	} else {
		fmt.Printf("✓ go mod tidy completed\n")
	}

	fmt.Printf("\n✓ Done! Your extracted package is ready at: %s\n", *output)
}

func getModuleInfo() (modulePath string, moduleDir string, err error) {
	// Get module path
	cmd := exec.Command("go", "list", "-m")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("go list -m failed: %w", err)
	}
	modulePath = strings.TrimSpace(string(out))

	// Get module directory
	cmd = exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	out, err = cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("go list -m -f failed: %w", err)
	}
	moduleDir = strings.TrimSpace(string(out))

	return modulePath, moduleDir, nil
}

func runGoda(expr string) ([]string, error) {
	// Try to find goda
	godaPath, err := findGoda()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(godaPath, "list", expr)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("goda failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var deps []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && line != "ID" { // Skip header
			deps = append(deps, line)
		}
	}

	return deps, nil
}

func findGoda() (string, error) {
	// Check if goda is in PATH
	if path, err := exec.LookPath("goda"); err == nil {
		return path, nil
	}

	// Check in GOPATH/bin
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}

	godaPath := filepath.Join(gopath, "bin", "goda")
	if _, err := os.Stat(godaPath); err == nil {
		return godaPath, nil
	}

	return "", fmt.Errorf("goda not found in PATH or %s", godaPath)
}

func copyDir(src, dst string) error {
	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Skip test directories, vendor, etc.
			if shouldSkipDir(entry.Name()) {
				continue
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Only copy Go files and important files
			if shouldCopyFile(entry.Name()) {
				if err := copyFile(srcPath, dstPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func shouldSkipDir(name string) bool {
	skip := []string{"testdata", "vendor", ".git", "_test"}
	for _, s := range skip {
		if name == s || strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

func shouldCopyFile(name string) bool {
	// Copy Go source files
	if strings.HasSuffix(name, ".go") {
		// Skip test files
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		return true
	}

	// Copy important non-Go files
	important := []string{"go.mod", "go.sum", "LICENSE", "README.md"}
	for _, f := range important {
		if name == f {
			return true
		}
	}

	return false
}

func replaceModuleInFiles(dir, oldModule, newModule string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only process .go files and go.mod
		name := info.Name()
		if !strings.HasSuffix(name, ".go") && name != "go.mod" {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Replace old module with new module
		newContent := strings.ReplaceAll(string(content), oldModule, newModule)

		// Only write if content changed
		if newContent != string(content) {
			if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
				return err
			}
		}

		return nil
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
