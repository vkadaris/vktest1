package codegen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestStepBoundaryLiterals searches for specific string literals in the project.
func TestStepBoundaryLiterals(t *testing.T) {
	projectRoot := `c:\dev\dmv\viya-data-flows`
	outputFile := `c:\dev\dmv\viya-data-flows\services\codegen\testdata\status_handling\step_boundaries.txt`
	stringLiterals := []string{
		"proc ",
		"data ",
		"filename ",
		"libname ",
	}

	excludeLiterals := []string{
		"loads data from",
		"updates data from",
		"in a data set",
	}

	exclusionList := ExclusionList{
		FilePatterns: []string{"*_test.go", "*abcd???xyz*.txt", "i18n_messages_*.go"}, // Example exclusion by file pattern
		Extensions:   []string{".txt", ".md", ".json", ".yaml", "*_test.go", ".exe"},  // Example exclusion by extension
		Directories: []string{
			filepath.Join(projectRoot, "vendor"), // Example exclusion by directory
			filepath.Join(projectRoot, "services", "codegen", "testdata"),
			filepath.Join(projectRoot, "services", "codetoflow", "testdata"),
			filepath.Join(projectRoot, "templates"),
			filepath.Join(projectRoot, "build"),
			filepath.Join(projectRoot, ".git"),
			filepath.Join(projectRoot, "docs"),
		}, // Example exclusion by directory
	}

	err := searchAndReport(projectRoot, stringLiterals, excludeLiterals, exclusionList, outputFile)
	if err != nil {
		t.Fatalf("Error during search: %v", err)
	}
	t.Log("String literal search completed successfully.")

}

// ExclusionList defines criteria for excluding files/directories during the search.
type ExclusionList struct {
	FilePatterns []string
	Extensions   []string
	Directories  []string
}

// searchAndReport performs the file traversal, string searching, and reporting.
func searchAndReport(projectRoot string, stringLiterals []string, excludeLiterals []string, exclusionList ExclusionList, outputFile string) error {
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer output.Close()

	// regex to remove comments
	commentRegex := regexp.MustCompile(`(?m)(//.*|/\*.*?\*/)`)

	excludedFiles := []string{}
	excludedDirectories := []string{}

	// Walk the project directory
	err = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Handle walk errors, like permission issues
			fmt.Fprintf(os.Stderr, "Error accessing path: %s, error: %v\n", path, err)
			return nil // Continue walking
		}
		// Skip directories
		if info.IsDir() {
			if isDirectoryExcluded(path, exclusionList.Directories) {
				fmt.Printf("Skipping directory: %s\n", path)
				excludedDirectories = append(excludedDirectories, path)
				return filepath.SkipDir // Skip this entire directory
			}
			return nil
		}

		// check for file exclusions
		if isFileExcluded(path, exclusionList.FilePatterns, exclusionList.Extensions) {
			fmt.Printf("Skipping file: %s\n", path)
			excludedFiles = append(excludedFiles, path)
			return nil // Skip this file
		}
		// Process the file
		err = processFile(path, stringLiterals, excludeLiterals, output, commentRegex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing file: %s, error: %v\n", path, err)
		}
		return nil

	})
	if err != nil {
		return fmt.Errorf("error during filepath.Walk: %w", err)
	}

	// Write excluded files and directories to the output file
	fmt.Fprintln(output, "\nExcluded Files:")
	for _, file := range excludedFiles {
		fmt.Fprintln(output, file)
	}

	fmt.Fprintln(output, "\nExcluded Directories:")
	for _, dir := range excludedDirectories {
		fmt.Fprintln(output, dir)
	}

	return nil
}

// processFile reads a file, searches for string literals, and reports findings.
func processFile(filePath string, stringLiterals []string, excludeLiterals []string, output io.Writer, commentRegex *regexp.Regexp) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	lineNumber := 1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break // End of file
			}
			return fmt.Errorf("error reading line: %w", err)
		}

		// Remove comments
		lineWithoutComments := commentRegex.ReplaceAllString(line, "")

		for _, literal := range stringLiterals {
			// Create a regex for word boundary matching
			re := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(literal)))
			matches := re.FindStringIndex(lineWithoutComments)

			if matches != nil {
				// Check if the line contains any exclude literals
				excludeMatch := false
				for _, excludeLiteral := range excludeLiterals {
					if strings.Contains(lineWithoutComments, excludeLiteral) {
						excludeMatch = true
						break
					}
				}
				// if not excluded, then report.
				if !excludeMatch {
					colNumber := matches[0]
					fmt.Printf("File: %s\n", filePath)
					fmt.Printf("  Line: %d, Column: %d\n", lineNumber, colNumber+1)
					fmt.Printf("  Match: %s\n", literal)
					fmt.Printf("  Line: %s\n", line)

					fmt.Fprintf(output, "File: %s\n", filePath)
					fmt.Fprintf(output, "  Line: %d, Column: %d\n", lineNumber, colNumber+1)
					fmt.Fprintf(output, "  Match: %s\n", literal)
					fmt.Fprintf(output, "  Line: %s\n", line)
				}
			}

		}
		lineNumber++
	}
	return nil
}

// isFileExcluded checks if a file should be excluded based on name or extension.
func isFileExcluded(filePath string, filePatterns []string, extensions []string) bool {
	fileName := filepath.Base(filePath)

	// Check for wildcard matches
	for _, pattern := range filePatterns {
		matched, err := filepath.Match(pattern, fileName) // match against file name
		if err == nil && matched {
			return true
		}
	}
	// Check for extension matches
	fileExt := filepath.Ext(fileName)
	for _, ext := range extensions {
		if fileExt == ext {
			return true
		}
	}
	return false
}

// isDirectoryExcluded checks if a directory should be excluded.
func isDirectoryExcluded(dirPath string, excludedDirs []string) bool {
	cleanedDirPath := filepath.Clean(dirPath)
	for _, excludedDir := range excludedDirs {
		cleanedExcludedDir := filepath.Clean(excludedDir)
		if strings.HasPrefix(cleanedDirPath, cleanedExcludedDir) {
			return true
		}
	}
	return false
}
