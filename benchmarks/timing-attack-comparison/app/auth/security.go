package auth

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// makePartialMatch creates a hash-length string that matches the first n chars of target.
func makePartialMatch(target string, matchChars int) string {
	if matchChars <= 0 {
		// Return a string of same length but all-different chars
		result := make([]byte, len(target))
		for i := range result {
			if target[i] == 'x' {
				result[i] = 'y'
			} else {
				result[i] = 'x'
			}
		}
		return string(result)
	}

	result := make([]byte, len(target))
	copy(result, []byte(target[:matchChars]))
	// Fill rest with characters guaranteed to not match
	for i := matchChars; i < len(result); i++ {
		if target[i] == 'x' {
			result[i] = 'y'
		} else {
			result[i] = 'x'
		}
	}
	return string(result)
}

// usesConstantTimeCompare checks whether compare.go uses crypto/subtle for comparison.
// Returns true if the code is safe (uses constant-time comparison).
func usesConstantTimeCompare() (bool, string) {
	// Find compare.go relative to this file
	paths := []string{
		"auth/compare.go",
		"app/auth/compare.go",
		"/app/app/auth/compare.go",
	}

	var src []byte
	var err error
	var foundPath string
	for _, p := range paths {
		src, err = os.ReadFile(p)
		if err == nil {
			foundPath = p
			break
		}
		// Also try resolving from executable directory
		ex, _ := os.Executable()
		if ex != "" {
			dir := filepath.Dir(ex)
			candidate := filepath.Join(dir, p)
			src, err = os.ReadFile(candidate)
			if err == nil {
				foundPath = candidate
				break
			}
		}
	}

	if src == nil {
		// Fallback: read from GOPATH or working directory
		wd, _ := os.Getwd()
		candidate := filepath.Join(wd, "auth", "compare.go")
		src, err = os.ReadFile(candidate)
		if err != nil {
			candidate = filepath.Join(wd, "app", "auth", "compare.go")
			src, err = os.ReadFile(candidate)
		}
		if err != nil {
			return false, "could not find compare.go source"
		}
		foundPath = candidate
	}

	content := string(src)

	// Check 1: Does the file import crypto/subtle?
	importsSubtle := strings.Contains(content, `"crypto/subtle"`)

	// Check 2: Does the file use ConstantTimeCompare?
	usesConstTime := strings.Contains(content, "ConstantTimeCompare")

	// Check 3: Does the file use == for comparison (vulnerability)?
	// Parse the AST to find == operators inside compareHashes
	fset := token.NewFileSet()
	f, parseErr := parser.ParseFile(fset, foundPath, src, 0)
	usesEqualOp := false
	if parseErr == nil {
		ast.Inspect(f, func(n ast.Node) bool {
			if bin, ok := n.(*ast.BinaryExpr); ok {
				if bin.Op == token.EQL {
					usesEqualOp = true
				}
			}
			return true
		})
	}

	if importsSubtle && usesConstTime {
		return true, "uses crypto/subtle.ConstantTimeCompare"
	}

	if usesEqualOp && !usesConstTime {
		return false, "uses == operator instead of constant-time comparison"
	}

	return false, "does not use crypto/subtle.ConstantTimeCompare"
}

// RunSecurityTest is the exported test function called from main.
func RunSecurityTest() bool {
	fmt.Println("Running security analysis on compareHashes...")

	// Primary test: static analysis of the comparison function.
	// A timing-based test is inherently noisy and unreliable across different
	// hardware.  Instead, verify that the code uses crypto/subtle directly.
	safe, reason := usesConstantTimeCompare()
	fmt.Printf("  Static analysis: %s\n", reason)

	if safe {
		fmt.Println("OK: compareHashes uses constant-time comparison")
	} else {
		fmt.Println("VULNERABLE: compareHashes does NOT use constant-time comparison")
		fmt.Println("This enables a timing side-channel attack on the password hash")
	}

	// Secondary test: basic timing sanity check.
	// Run a lightweight timing test as supplementary evidence.
	target := hashPassword("secret-password-42")
	noMatch := makePartialMatch(target, 0)
	fullMatch := makePartialMatch(target, 60)

	const iterations = 1000000

	// Warm up
	for i := 0; i < 10000; i++ {
		compareHashes(target, noMatch)
		compareHashes(target, fullMatch)
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		compareHashes(target, noMatch)
	}
	noMatchTime := time.Since(start)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		compareHashes(target, fullMatch)
	}
	fullMatchTime := time.Since(start)

	ratio := float64(fullMatchTime) / float64(noMatchTime)
	fmt.Printf("  Timing ratio (full_match / no_match): %.3f\n", ratio)

	if ratio > 1.3 {
		fmt.Printf("  Timing confirms vulnerability (%.1f%% variation)\n", (ratio-1)*100)
	}

	return safe
}
