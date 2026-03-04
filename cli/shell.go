//go:build !js

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

func splitArgs(line string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	escape := false
	for _, r := range line {
		switch {
		case escape:
			cur.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuote = r
		case unicode.IsSpace(r):
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if inQuote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if escape {
		return nil, fmt.Errorf("dangling escape")
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, nil
}

func repl(a *App) error {
	fmt.Println("Bao CLI - interactive mode (type 'help' for commands, 'tui' for full-screen mode)")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("bao> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return nil
		}
		parts, err := splitArgs(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			continue
		}
		if err := a.execute(parts); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}
}
