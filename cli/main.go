//go:build !js

package main

import (
	"fmt"
	"os"
)

func main() {
	a := newApp()
	defer a.closeAll()

	printStartupLogo()
	a.printIdentitySummary()
	fmt.Println()

	if len(os.Args) == 1 {
		if err := a.ensurePrivateIDAtStartup(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		a.printIdentitySummary()
		fmt.Println()
	}

	if len(os.Args) > 1 {
		if os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
			a.help()
			return
		}
		if err := a.execute(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if err := repl(a); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
