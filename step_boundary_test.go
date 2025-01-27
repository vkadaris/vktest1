package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// SearchResult struct to hold the search result information
type SearchResult struct {
	Filename   string
	LineNumber int
	ColumnNumber int
	Match      string
}

// findStringLiterals searches for string literals in a file, ignoring comments, and returns a slice of SearchResult.
func findStringLiterals(filename string, stringLiterals []string) ([]SearchResult, error) {
	results := []SearchResult{}
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	inCommentBlock := false
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Handle multi-line comments
		if strings.Contains(line, "/*") {
			inCommentBlock = true
		}

		if strings.Contains(line, "*/") {
			inCommentBlock = false
			continue // Skip processing this line entirely after closing comment block.
		}

		// Skip lines within multi-line comments or single-line comments.
		if inCommentBlock || strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}

		// Search for string literals in non-comment lines
		for _, literal := range stringLiterals {
			if index := strings.Index(line, literal); index != -1 {
				results = append(results, SearchResult{
					Filename:   filename,
					LineNumber: lineNumber,
					ColumnNumber: index + 1, // Column number is 1-based
					Match:      literal,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file %s: %w", filename, err)
	}

	return results, nil
}

// ExclusionReason struct to hold the exclusion reason.
type ExclusionReason struct {
	Filename string
	Reason   string
}

// shouldExclude checks if a file or directory should be excluded based on the exclusion list, returns a reason if excluded.
func shouldExclude(path string, exclusionList []string) (bool, string) {
	for _, exclusion := range exclusionList {
		if strings.HasSuffix(path, exclusion) {
			return true, fmt.Sprintf("Excluded due to suffix match: %s", exclusion) // Exclude based on suffix match (extension or filename)
		}
		if path == exclusion {
			return true, fmt.Sprintf("Excluded due to exact match: %s", exclusion) // Exclude based on exact path match
		}
	}
	return false, ""
}

// traverseAndSearch traverses the directory, searches for string literals, and reports the findings.
func traverseAndSearch(root string, stringLiterals []string, exclusionList []string, outputFile string) ([]SearchResult, []ExclusionReason, error) {
	var allResults []SearchResult
    var allExclusions []ExclusionReason
	var mu sync.Mutex  // Mutex to protect shared resources
	var wg sync.WaitGroup // WaitGroup to wait for all goroutines to complete

	// Open output file for writing
	file, err := os.Create(outputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating output file %s: %w", outputFile, err)
	}
	defer file.Close()

    // Add header to output file
    _, err = file.WriteString("Filename,LineNumber,ColumnNumber,Match\n")
    if err != nil {
        return nil, nil, fmt.Errorf("error writing header to output file: %w", err)
    }

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err // Returning the error prevents further traversal. Consider logging only if you want to continue.
		}

		excluded, reason := shouldExclude(path, exclusionList)
		if excluded {
			if info.IsDir() {
                allExclusions = append(allExclusions, ExclusionReason{Filename: path, Reason: reason})
                fmt.Printf("Skipping directory: %s, Reason: %s\n", path, reason)
				return filepath.SkipDir // Skip the directory if it's excluded
			}

            allExclusions = append(allExclusions, ExclusionReason{Filename: path, Reason: reason})
            fmt.Printf("Skipping file: %s, Reason: %s\n", path, reason)
			return nil // Skip the file
		}

		if !info.IsDir() && filepath.Ext(path) == ".go" {
			wg.Add(1) // Increment waitgroup counter for each goroutine spawned.

			// Use a goroutine to process each file concurrently
			go func(filePath string) {
				defer wg.Done()
				results, err := findStringLiterals(filePath, stringLiterals)
				if err != nil {
					fmt.Printf("error searching file %s: %v\n", filePath, err)
					return
				}

				// Use a Mutex to safely append to the allResults slice and write to the file
				mu.Lock()
				defer mu.Unlock()

				allResults = append(allResults, results...)

                for _, result := range results {
                    fmt.Printf("  File: %s, Line: %d, Column: %d, Match: %s\n", result.Filename, result.LineNumber, result.ColumnNumber, result.Match)
                    _, err := file.WriteString(fmt.Sprintf("%s,%d,%d,%s\n", result.Filename, result.LineNumber, result.ColumnNumber, result.Match))
                    if err != nil {
                        fmt.Printf("Error writing to output file: %v\n", err)
                    }
                }
			}(path)
		}

		return nil
	})

	wg.Wait() // Wait for all goroutines to finish

	if err != nil {
		return nil, nil, fmt.Errorf("error walking the path %s: %w", root, err)
	}

	return allResults, allExclusions, nil
}

// TestFileSearchWithExclusion is the test function that performs the string literal search with exclusion reporting.
func TestFileSearchWithExclusion(t *testing.T) {
    root := "c:\\dev\\dmv\\viya-data-flows"
	stringLiterals := []string{"TODO:", "FIXME:", "panic("}
	exclusionList := []string{".txt", "vendor/", "main.go"}
    outputFile := "search_results.csv"

	results, exclusions, err := traverseAndSearch(root, stringLiterals, exclusionList, outputFile)
	if err != nil {
		t.Fatalf("Error during traversal and search: %v", err)
	}

	// Report the findings. You can customize the reporting format as needed.
    if len(results) == 0 && len(exclusions) == 0{
        t.Logf("No string literals found in the project, and no files excluded (excluding excluded files and comments).")
    } else if len(results) == 0 {
		t.Logf("No string literals found in the project (excluding excluded files and comments).")
    } else {
		t.Logf("String literals found:")
        // Results are already printed to the console and written to file in traverseAndSearch function
    }

    if len(exclusions) > 0 {
        t.Logf("Files and Directories Excluded:")
        for _, exclusion := range exclusions {
            t.Logf(" File: %s, Reason: %s\n", exclusion.Filename, exclusion.Reason)
        }
    }

}
// --- original stepBoundary Test Code ---

func stepBoundary() {
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			fmt.Println("Step 1: ", i)
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Println("Step 1 finished")
	}()

	go func() {
		wg.Wait()
		fmt.Println("Step 2 Starting...")
		for j := 0; j < 5; j++ {
			fmt.Println("Step 2: ", j)
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Println("Step 2 finished")
	}()

	wg.Wait()
}

func TestStepBoundary(t *testing.T) {
	t.Logf("Starting...")
	stepBoundary()
	t.Logf("Finished...")
}

// --- end original stepBoundary Test Code ---

// Main function to run the test (optional, useful for debugging outside of `go test`).
func main() {
	testing.Main(nil, nil, nil) // Use testing.Main to run the tests.
}