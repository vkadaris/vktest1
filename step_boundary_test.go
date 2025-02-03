package codegen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
		"for sca proc code execution",
		" data into",
		"in proc python the",
		" using proc ",
		"a data flow",
		"data set options are",
		"data set contains",
		"rows in the data set",
		"data sets only",
		"one or more SAS data",
		"array of SAS data",
		"requires a proc contents",
		"as data step",
		"generates data flow",
		" data action",
		"data flow step",
		"data step in CAS utility",
		"operations in a data flow",
		" proc casutil utility ",
		"set as data set options",
		"different data providers",
		"data flow service uses",
		"(data view)",
		"is data step",
	}

	exclusionList := ExclusionList{
		FilePatterns: []string{"*_test.go", "*abcd???xyz*.txt", "i18n_messages_*.go"}, // file patterns to exclude
		Extensions:   []string{".txt", ".md", ".json", ".yaml", "*_test.go", ".exe"},  // file extensions to exclude
		Directories: []string{
			filepath.Join(projectRoot, "vendor"), // Example exclusion by directory
			filepath.Join(projectRoot, "services", "codegen", "testdata"),
			filepath.Join(projectRoot, "services", "codetoflow", "testdata"),
			filepath.Join(projectRoot, "templates"),
			filepath.Join(projectRoot, "build"),
			filepath.Join(projectRoot, ".git"),
			filepath.Join(projectRoot, "docs"),
		}, // directories to exclude
	}

	err := searchAndReport(projectRoot, stringLiterals, excludeLiterals, exclusionList, outputFile)
	if err != nil {
		t.Fatalf("Error during search: %v", err)
	}
	t.Log("String literal search completed successfully.")

}

// ExclusionList defines criteria for excluding file patterns, file extensions and directories during the search.
type ExclusionList struct {
	FilePatterns []string
	Extensions   []string
	Directories  []string
}

// ExclusionReason struct to hold the exclusion reason.
type ExclusionReason struct {
	Path   string
	Reason string
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

	excludedItems := []ExclusionReason{}
	fileMatches := make(map[string][]string) // To store string literal matches per file

	// Walk the project directory
	err = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Handle walk errors, like permission issues
			fmt.Fprintf(os.Stderr, "Error accessing path: %s, error: %v\n", path, err)
			return nil // Continue walking
		}
		// Skip directories
		if info.IsDir() {
			if reason, excluded := isDirectoryExcluded(path, exclusionList.Directories); excluded {
				fmt.Printf("Skipping directory: %s, Reason: %s\n", path, reason)
				excludedItems = append(excludedItems, ExclusionReason{Path: path, Reason: reason})
				return filepath.SkipDir // Skip this entire directory
			}
			return nil
		}

		// check for file exclusions
		if reason, excluded := isFileExcluded(path, exclusionList.FilePatterns, exclusionList.Extensions); excluded {
			fmt.Printf("Skipping file: %s, Reason: %s\n", path, reason)
			excludedItems = append(excludedItems, ExclusionReason{Path: path, Reason: reason})
			return nil // Skip this file
		}

		// Process the file
		matches, err := processFile(path, stringLiterals, excludeLiterals, commentRegex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing file: %s, error: %v\n", path, err)
		}
		if len(matches) > 0 {
			fileMatches[path] = matches
		}
		return nil

	})
	if err != nil {
		return fmt.Errorf("error during filepath.Walk: %w", err)
	}

	// Write excluded files and directories to the output file
	fmt.Fprintln(output, "\nExcluded Files and Directories:")
	for _, item := range excludedItems {
		fmt.Fprintf(output, "Path: %s, Reason: %s\n", item.Path, item.Reason)
	}

	fmt.Fprintln(output, "\nString Literal Matches:")

	var files []string
	for file := range fileMatches {
		files = append(files, file)
	}
	sort.Strings(files)
	// Write the matches group by file.
	for _, file := range files {
		fmt.Fprintf(output, "File: %s\n", file)
		for _, match := range fileMatches[file] {
			fmt.Fprintln(output, match)
		}
	}

	// Add Summary
	fmt.Fprintln(output, "\nSummary:")
	for _, file := range files {
		fmt.Fprintf(output, "File: %s, Matches Found: %d\n", file, len(fileMatches[file]))
	}

	return nil
}

// processFile reads a file, searches for string literals, and reports findings.
func processFile(filePath string, stringLiterals []string, excludeLiterals []string, commentRegex *regexp.Regexp) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	lineNumber := 1
	var matches []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break // End of file
			}
			return nil, fmt.Errorf("error reading line: %w", err)
		}

		// Remove comments
		lineWithoutComments := commentRegex.ReplaceAllString(line, "")

		for _, literal := range stringLiterals {
			// Create a regex for word boundary matching
			re := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(literal)))
			matchIndexes := re.FindStringIndex(lineWithoutComments)

			if matchIndexes != nil {
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
					colNumber := matchIndexes[0]

					matchStr := fmt.Sprintf("  Row: %d, Column: %d\n", lineNumber, colNumber+1)
					matchStr = matchStr + fmt.Sprintf("  Match: %s\n", literal)
					matchStr = matchStr + fmt.Sprintf("  Line: %s\n", line)
					fmt.Printf("File: %s\n", filePath)
					fmt.Printf(matchStr)
					matches = append(matches, matchStr)
				}
			}

		}
		lineNumber++
	}
	return matches, nil
}

// isFileExcluded checks if a file should be excluded based on name or extension.
func isFileExcluded(filePath string, filePatterns []string, extensions []string) (string, bool) {
	fileName := filepath.Base(filePath)

	// Check for wildcard matches
	for _, pattern := range filePatterns {
		matched, err := filepath.Match(pattern, fileName) // match against file name
		if err == nil && matched {
			return fmt.Sprintf("Matched file pattern: %s", pattern), true
		}
	}
	// Check for extension matches
	fileExt := filepath.Ext(fileName)
	for _, ext := range extensions {
		if fileExt == ext {
			return fmt.Sprintf("Matched extension: %s", ext), true
		}
	}
	return "", false
}

// isDirectoryExcluded checks if a directory should be excluded.
func isDirectoryExcluded(dirPath string, excludedDirs []string) (string, bool) {
	cleanedDirPath := filepath.Clean(dirPath)
	for _, excludedDir := range excludedDirs {
		cleanedExcludedDir := filepath.Clean(excludedDir)
		if strings.HasPrefix(cleanedDirPath, cleanedExcludedDir) {
			return fmt.Sprintf("Matched directory: %s", excludedDir), true
		}
	}
	return "", false
}
