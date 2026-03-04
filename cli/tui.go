//go:build !js

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/vault"
	"golang.org/x/term"
)

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiCyan      = "\x1b[36m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiMagenta   = "\x1b[35m"
	ansiRed       = "\x1b[31m"
	ansiBlue      = "\x1b[34m"
	ansiWhite     = "\x1b[37m"
	ansiFgBright  = "\x1b[97m"
	ansiBgBlack   = "\x1b[40m"
	ansiBgBlue    = "\x1b[44m"
	ansiBgCyan    = "\x1b[46m"
	ansiBgMagenta = "\x1b[45m"
)

type tuiPane int

const (
	paneVault tuiPane = iota
	paneReplica
)

type tuiState struct {
	pane tuiPane

	files      []vault.File
	fileSel    int
	tables     []string
	tableSel   int
	rows       [][]any
	status     string
	help       bool
	lastRender time.Time
}

func (a *App) runTUI() error {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(fd, old) }()

	fmt.Print("\x1b[?1049h\x1b[?25l")
	defer fmt.Print("\x1b[?25h\x1b[?1049l")

	s := &tuiState{pane: paneVault, status: "TUI started"}
	_ = a.tuiRefresh(s)
	_ = a.tuiRender(s)

	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		b := buf[:n]

		if len(b) == 1 {
			switch b[0] {
			case 'q', 'Q':
				return nil
			case '?':
				s.help = !s.help
			case '\t':
				if s.pane == paneVault {
					s.pane = paneReplica
				} else {
					s.pane = paneVault
				}
			case '1':
				s.pane = paneVault
			case '2':
				s.pane = paneReplica
			case 'r', 'R':
				if err := a.tuiRefresh(s); err != nil {
					s.status = "refresh error: " + err.Error()
				}
			case 's', 'S':
				if s.pane == paneVault {
					if err := a.cmdSync(nil); err != nil {
						s.status = "sync error: " + err.Error()
					} else {
						s.status = "vault sync done"
						_ = a.tuiRefresh(s)
					}
				} else {
					if err := a.cmdReplicaSync(nil); err != nil {
						s.status = "replica sync error: " + err.Error()
					} else {
						s.status = "replica sync done"
						_ = a.tuiRefresh(s)
					}
				}
			case 'o', 'O':
				if err := a.cmdOpen([]string{"--private", a.session.PrivateID, "--creator", a.session.CreatorPublic, "--config", a.session.StoreConfig, "--db", a.session.VaultDBPath}); err != nil {
					s.status = "open error: " + err.Error()
				} else {
					s.status = "vault opened"
					_ = a.tuiRefresh(s)
				}
			case 'p', 'P':
				args := []string{"--db", a.session.ReplicaDBPath, "--dir", a.session.ReplicaDir}
				if strings.TrimSpace(a.session.ReplicaDDLPath) != "" {
					args = append(args, "--ddl", a.session.ReplicaDDLPath)
				}
				if err := a.cmdReplicaOpen(args); err != nil {
					s.status = "replica open error: " + err.Error()
				} else {
					s.status = "replica opened"
					_ = a.tuiRefresh(s)
				}
			case 'j':
				a.tuiMoveSel(s, +1)
			case 'k':
				a.tuiMoveSel(s, -1)
			case 'u', 'U':
				if s.pane == paneVault {
					_ = a.cmdCd([]string{".."})
					_ = a.tuiRefresh(s)
					s.status = "moved up"
				}
			case 13:
				a.tuiEnter(s)
			}
		} else if len(b) >= 3 && b[0] == 27 && b[1] == '[' {
			switch b[2] {
			case 'A':
				a.tuiMoveSel(s, -1)
			case 'B':
				a.tuiMoveSel(s, +1)
			}
		}
		_ = a.tuiRender(s)
	}
}

func (a *App) tuiMoveSel(s *tuiState, delta int) {
	if s.pane == paneVault {
		n := len(s.files)
		if n == 0 {
			s.fileSel = 0
			return
		}
		s.fileSel += delta
		if s.fileSel < 0 {
			s.fileSel = 0
		}
		if s.fileSel >= n {
			s.fileSel = n - 1
		}
		return
	}
	n := len(s.tables)
	if n == 0 {
		s.tableSel = 0
		return
	}
	s.tableSel += delta
	if s.tableSel < 0 {
		s.tableSel = 0
	}
	if s.tableSel >= n {
		s.tableSel = n - 1
	}
}

func (a *App) tuiEnter(s *tuiState) {
	if s.pane == paneVault {
		if len(s.files) == 0 {
			return
		}
		f := s.files[s.fileSel]
		if f.IsDir {
			a.cwd = normalizeVaultPath(f.Name)
			_ = a.tuiRefresh(s)
			s.status = "opened dir /" + a.cwd
			return
		}
		local := filepath.Base(f.Name)
		_, err := a.v.Read(f.Name, local, vault.IOOption{}, nil)
		if err != nil {
			s.status = "download error: " + err.Error()
			return
		}
		s.status = fmt.Sprintf("downloaded %s -> %s", f.Name, local)
		return
	}
	if len(s.tables) == 0 || a.replica == nil {
		return
	}
	t := s.tables[s.tableSel]
	q := fmt.Sprintf("SQL:SELECT * FROM \"%s\" LIMIT 100", t)
	rows, err := a.replica.Fetch(q, sqlx.Args{}, 100)
	if err != nil {
		s.status = "preview error: " + err.Error()
		return
	}
	s.rows = rows
	s.status = fmt.Sprintf("preview %s (%d rows)", t, len(rows))
}

func (a *App) tuiRefresh(s *tuiState) error {
	if a.v != nil {
		files, err := a.v.ReadDir(a.cwd, time.Time{}, 0, 500)
		if err == nil {
			s.files = files
			if s.fileSel >= len(s.files) {
				s.fileSel = len(s.files) - 1
				if s.fileSel < 0 {
					s.fileSel = 0
				}
			}
		} else {
			s.status = "ls error: " + err.Error()
		}
	}
	if a.replica != nil {
		rows, err := a.replica.Fetch("SQL:SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name", sqlx.Args{}, 1000)
		if err == nil {
			list := make([]string, 0, len(rows))
			for _, r := range rows {
				if len(r) > 0 {
					list = append(list, fmt.Sprintf("%v", r[0]))
				}
			}
			s.tables = list
			if s.tableSel >= len(s.tables) {
				s.tableSel = len(s.tables) - 1
				if s.tableSel < 0 {
					s.tableSel = 0
				}
			}
		} else {
			s.status = "tables error: " + err.Error()
		}
	}
	return nil
}

func termSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 || h == 0 {
		return 120, 40
	}
	return w, h
}

func clip(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func center(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return clip(s, w)
	}
	pad := (w - len(r)) / 2
	return strings.Repeat(" ", pad) + s
}

func baoLogoASCII() []string {
	return []string{
		"             __",
		"         _.-'  `-._",
		"      .-'  .--.    '-.",
		"    .'   .'_  _.'.    '.",
		"   /    / (o)(o) \\     \\",
		"  ;    |    __    |     ;",
		"  |    |  .'__`.  |     |",
		"  ;     \\ `-__-' /      ;",
		"   \\      `----'      /",
		"    '.              .'",
		"      '-.________.-'",
		"       /   010100   \\",
		"      /______________\\",
	}
}

func printStartupLogo() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	w, _ := termSize()
	if w < 24 {
		fmt.Println("Bao CLI")
		return
	}
	logo := baoLogoASCII()
	for i, ln := range logo {
		clr := ansiYellow
		if i >= len(logo)-2 {
			clr = ansiCyan
		}
		fmt.Println(clr + center(clip(ln, w), w) + ansiReset)
	}
	fmt.Println(ansiBold + ansiBlue + center("Bao CLI", w) + ansiReset)
	fmt.Println(ansiDim + center("Secure Vault + Replica Terminal", w) + ansiReset)
	fmt.Println()
}

func (a *App) tuiRender(s *tuiState) error {
	w, h := termSize()
	if w < 20 {
		w = 20
	}
	if h < 6 {
		h = 6
	}
	var b bytes.Buffer
	b.WriteString("\x1b[H\x1b[2J")

	title := "Bao TUI  [q quit] [tab/1/2 switch] [o open vault] [p open replica] [r refresh] [s sync] [j/k or arrows move] [enter action] [u up] [? help]"
	if s.pane == paneVault {
		title += "  | Pane: Vault"
	} else {
		title += "  | Pane: Replica"
	}
	b.WriteString(ansiBold + ansiFgBright + ansiBgBlue + clip(title, w) + ansiReset + "\r\n")
	if w >= 26 {
		vaultTab := ansiDim + ansiWhite + ansiBgBlack + " Vault " + ansiReset
		replicaTab := ansiDim + ansiWhite + ansiBgBlack + " Replica " + ansiReset
		if s.pane == paneVault {
			vaultTab = ansiBold + ansiFgBright + ansiBgCyan + " Vault " + ansiReset
		} else {
			replicaTab = ansiBold + ansiFgBright + ansiBgMagenta + " Replica " + ansiReset
		}
		b.WriteString(vaultTab + " " + replicaTab + "\r\n")
	} else {
		paneName := "Vault"
		if s.pane == paneReplica {
			paneName = "Replica"
		}
		b.WriteString(ansiBold + ansiCyan + "Pane: " + paneName + ansiReset + "\r\n")
	}
	b.WriteString(ansiDim + ansiCyan + clip(strings.Repeat("=", w), w) + ansiReset + "\r\n")

	extraTop := 0
	logo := baoLogoASCII()
	showLogo := w >= 56 && h >= 24
	if showLogo {
		for i, ln := range logo {
			clr := ansiYellow
			if i >= len(logo)-2 {
				clr = ansiCyan
			}
			b.WriteString(clr + center(ln, w) + ansiReset + "\r\n")
		}
		b.WriteString(ansiDim + center("Bao Secure Vault", w) + ansiReset + "\r\n")
		extraTop = len(logo) + 1
	}

	if s.help {
		help := []string{
			"Vault pane:",
			"  enter on dir = open dir; enter on file = download to local cwd",
			"Replica pane:",
			"  enter on table = preview SELECT * LIMIT 100",
			"Setup:",
			"  open uses saved session (~/.bao/cli-session.yaml)",
			"  replica-open uses saved replica settings",
		}
		for idx, ln := range help {
			line := clip(ln, w)
			if strings.HasSuffix(line, ":") {
				line = ansiBold + ansiMagenta + line + ansiReset
			} else if idx >= len(help)-2 {
				line = ansiDim + line + ansiReset
			}
			b.WriteString(line + "\r\n")
		}
		contentRows := h - 5 - extraTop
		if contentRows < 0 {
			contentRows = 0
		}
		for i := len(help); i < contentRows; i++ {
			b.WriteString("\r\n")
		}
	} else {
		leftW := w / 2
		rightW := w - leftW - 1
		if rightW < 1 {
			rightW = 1
		}
		rows := h - 5 - extraTop
		if rows < 0 {
			rows = 0
		}
		for i := 0; i < rows; i++ {
			left := ""
			right := ""
			selected := false
			if s.pane == paneVault {
				if i == 0 {
					left = "Vault: /" + a.cwd
					right = "Details"
				} else {
					idx := i - 1
					if idx < len(s.files) {
						f := s.files[idx]
						mark := " "
						if idx == s.fileSel {
							mark = ">"
							selected = true
						}
						typ := "F"
						if f.IsDir {
							typ = "D"
						}
						left = fmt.Sprintf("%s [%s] %s", mark, typ, f.Name)
					}
					if len(s.files) > 0 {
						f := s.files[s.fileSel]
						switch i {
						case 1:
							right = "name: " + f.Name
						case 2:
							right = fmt.Sprintf("size: %d", f.Size)
						case 3:
							right = "mod : " + f.ModTime.Format(time.RFC3339)
						case 4:
							right = "attrs: " + attrsPreview(f.Attrs)
						}
					}
				}
			} else {
				if i == 0 {
					left = "Replica tables"
					right = "Rows preview"
				} else {
					idx := i - 1
					if idx < len(s.tables) {
						mark := " "
						if idx == s.tableSel {
							mark = ">"
							selected = true
						}
						left = fmt.Sprintf("%s %s", mark, s.tables[idx])
					}
					if idx < len(s.rows) {
						right = rowToText(s.rows[idx])
					}
				}
			}
			line := fmt.Sprintf("%-*s| %-*s", leftW, clip(left, leftW), rightW, clip(right, rightW))
			if i == 0 {
				line = ansiBold + ansiBlue + line + ansiReset
			}
			if selected {
				line = ansiBold + ansiFgBright + ansiBgBlue + line + ansiReset
			} else if strings.Contains(line, "[D]") {
				line = ansiCyan + line + ansiReset
			}
			b.WriteString(line + "\r\n")
		}
	}

	b.WriteString(ansiDim + ansiCyan + clip(strings.Repeat("-", w), w) + ansiReset + "\r\n")
	status := s.status
	if status == "" {
		status = "ready"
	}
	statusLine := clip(status, w)
	lcStatus := strings.ToLower(status)
	if strings.Contains(lcStatus, "error") {
		statusLine = ansiBold + ansiRed + statusLine + ansiReset
	} else if strings.Contains(lcStatus, "done") || strings.Contains(lcStatus, "opened") || strings.Contains(lcStatus, "downloaded") {
		statusLine = ansiBold + ansiGreen + statusLine + ansiReset
	} else {
		statusLine = ansiBold + ansiYellow + statusLine + ansiReset
	}
	b.WriteString(statusLine + "\r\n")

	_, err := os.Stdout.Write(b.Bytes())
	s.lastRender = time.Now()
	return err
}

func rowToText(r []any) string {
	parts := make([]string, 0, len(r))
	for _, v := range r {
		if v == nil {
			parts = append(parts, "NULL")
			continue
		}
		switch x := v.(type) {
		case []byte:
			parts = append(parts, attrsPreview(x))
		default:
			parts = append(parts, fmt.Sprintf("%v", x))
		}
	}
	return strings.Join(parts, " | ")
}
