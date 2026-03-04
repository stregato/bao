//go:build !js

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/stregato/bao/lib/replica"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/stregato/bao/lib/vault"
	"golang.org/x/term"
)

func promptArrowChoice(title string, options []string, initial int) (int, error) {
	if len(options) == 0 {
		return -1, fmt.Errorf("no options")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return -1, fmt.Errorf("interactive terminal required")
	}
	if initial < 0 || initial >= len(options) {
		initial = 0
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return -1, err
	}
	defer func() { _ = term.Restore(fd, oldState) }()
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	selected := initial
	lines := len(options) + 1
	render := func(first bool) {
		if !first {
			fmt.Printf("\x1b[%dA", lines)
		}
		fmt.Print("\x1b[2K\r")
		fmt.Printf("%s (use ↑/↓, Enter)\n", title)
		for i, opt := range options {
			fmt.Print("\x1b[2K\r")
			if i == selected {
				fmt.Printf("  > %s\n", opt)
			} else {
				fmt.Printf("    %s\n", opt)
			}
		}
	}

	render(true)
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return -1, err
		}
		if n == 0 {
			continue
		}
		b := buf[:n]
		switch {
		case n == 1 && (b[0] == '\r' || b[0] == '\n'):
			return selected, nil
		case n == 1 && (b[0] == 'k' || b[0] == 'K'):
			if selected > 0 {
				selected--
				render(false)
			}
		case n == 1 && (b[0] == 'j' || b[0] == 'J'):
			if selected < len(options)-1 {
				selected++
				render(false)
			}
		case n >= 3 && b[0] == 27 && b[1] == '[' && b[2] == 'A':
			if selected > 0 {
				selected--
				render(false)
			}
		case n >= 3 && b[0] == 27 && b[1] == '[' && b[2] == 'B':
			if selected < len(options)-1 {
				selected++
				render(false)
			}
		case n == 1 && b[0] == 3:
			return -1, fmt.Errorf("interrupted")
		}
	}
}

func promptPrivateID(current string) (string, error) {
	current = strings.TrimSpace(current)
	if current != "" {
		return current, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("--private is required in non-interactive mode")
	}
	r := bufio.NewReader(os.Stdin)
	for {
		choice, err := promptArrowChoice("Private ID is not set", []string{
			"Import private ID",
			"Generate new private ID",
		}, 0)
		if err != nil {
			return "", err
		}
		fmt.Println()
		switch choice {
		case 0:
			fmt.Print("Paste private ID: ")
			idRaw, err := r.ReadString('\n')
			if err != nil {
				return "", err
			}
			id := strings.TrimSpace(idRaw)
			if id == "" {
				fmt.Println("Private ID cannot be empty.")
				continue
			}
			return id, nil
		case 1:
			id, err := security.NewPrivateID()
			if err != nil {
				return "", err
			}
			pub, err := id.PublicID()
			if err != nil {
				return "", err
			}
			fmt.Printf("Generated private ID: %s\n", id)
			fmt.Printf("Derived public ID:  %s\n", pub)
			return string(id), nil
		}
	}
}

func promptCreatorPublicID(current string) (string, error) {
	current = strings.TrimSpace(current)
	if current != "" {
		return current, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("--creator is required in non-interactive mode")
	}
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Creator public ID: ")
		v, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			fmt.Println("Creator public ID cannot be empty.")
			continue
		}
		return v, nil
	}
}

func promptConfigPath(current string) (string, error) {
	current = strings.TrimSpace(current)
	validate := func(p string) error {
		if p == "" {
			return fmt.Errorf("config path is empty")
		}
		st, err := os.Stat(p)
		if err != nil {
			return err
		}
		if st.IsDir() {
			return fmt.Errorf("config path points to a directory")
		}
		return nil
	}
	if current != "" {
		if err := validate(current); err == nil {
			return current, nil
		}
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("--config is required in non-interactive mode")
	}

	r := bufio.NewReader(os.Stdin)
	for {
		choice, err := promptArrowChoice("Store config file is not set", []string{
			"Type path manually",
			"Browse file system",
		}, 0)
		if err != nil {
			return "", err
		}
		fmt.Println()
		var p string
		switch choice {
		case 0:
			fmt.Print("Config file path: ")
			raw, err := r.ReadString('\n')
			if err != nil {
				return "", err
			}
			p = strings.TrimSpace(raw)
		case 1:
			picked, err := browseForFile(".")
			if err != nil {
				return "", err
			}
			if picked == "" {
				fmt.Println("Selection cancelled.")
				continue
			}
			p = picked
		}
		if err := validate(p); err != nil {
			fmt.Printf("Invalid config path: %v\n", err)
			continue
		}
		return p, nil
	}
}

type pickerItem struct {
	name  string
	path  string
	isDir bool
}

func browseForFile(startDir string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("interactive terminal required for browsing")
	}
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	cwd := abs
	selected := 0
	offset := 0

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(fd, oldState) }()
	fmt.Print("\x1b[?1049h\x1b[?25l")
	defer fmt.Print("\x1b[?25h\x1b[?1049l")

	readItems := func(dir string) ([]pickerItem, error) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		dirs := make([]pickerItem, 0)
		files := make([]pickerItem, 0)
		parent := filepath.Dir(dir)
		if parent != dir {
			dirs = append(dirs, pickerItem{name: "..", path: parent, isDir: true})
		}
		for _, e := range ents {
			it := pickerItem{name: e.Name(), path: filepath.Join(dir, e.Name()), isDir: e.IsDir()}
			if it.isDir {
				dirs = append(dirs, it)
			} else {
				files = append(files, it)
			}
		}
		sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name) })
		sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].name) < strings.ToLower(files[j].name) })
		return append(dirs, files...), nil
	}

	items, err := readItems(cwd)
	if err != nil {
		return "", err
	}

	render := func() {
		w, h, _ := term.GetSize(int(os.Stdout.Fd()))
		if w < 60 {
			w = 60
		}
		if h < 12 {
			h = 12
		}
		visible := h - 5
		if visible < 3 {
			visible = 3
		}
		if selected < offset {
			offset = selected
		}
		if selected >= offset+visible {
			offset = selected - visible + 1
		}
		fmt.Print("\x1b[H\x1b[2J")
		fmt.Printf("Select config file (↑/↓ move, Enter select/open, Backspace up, q cancel)\n")
		fmt.Printf("Current: %s\n", cwd)
		fmt.Println(strings.Repeat("-", w))
		if len(items) == 0 {
			fmt.Println("(empty)")
			return
		}
		end := offset + visible
		if end > len(items) {
			end = len(items)
		}
		for i := offset; i < end; i++ {
			it := items[i]
			marker := " "
			if i == selected {
				marker = ">"
			}
			kind := "F"
			if it.isDir {
				kind = "D"
			}
			fmt.Printf("%s [%s] %s\n", marker, kind, it.name)
		}
	}

	render()
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		b := buf[:n]
		switch {
		case n == 1 && (b[0] == 'q' || b[0] == 'Q' || b[0] == 27):
			return "", nil
		case n == 1 && (b[0] == '\r' || b[0] == '\n'):
			if len(items) == 0 {
				continue
			}
			it := items[selected]
			if it.isDir {
				cwd = it.path
				selected = 0
				offset = 0
				items, err = readItems(cwd)
				if err != nil {
					return "", err
				}
				render()
				continue
			}
			return it.path, nil
		case n == 1 && (b[0] == 127 || b[0] == 8):
			parent := filepath.Dir(cwd)
			if parent != cwd {
				cwd = parent
				selected = 0
				offset = 0
				items, err = readItems(cwd)
				if err != nil {
					return "", err
				}
				render()
			}
		case n == 1 && (b[0] == 'k' || b[0] == 'K'):
			if selected > 0 {
				selected--
				render()
			}
		case n == 1 && (b[0] == 'j' || b[0] == 'J'):
			if selected < len(items)-1 {
				selected++
				render()
			}
		case n >= 3 && b[0] == 27 && b[1] == '[' && b[2] == 'A':
			if selected > 0 {
				selected--
				render()
			}
		case n >= 3 && b[0] == 27 && b[1] == '[' && b[2] == 'B':
			if selected < len(items)-1 {
				selected++
				render()
			}
		}
	}
}

func (a *App) ensurePrivateIDAtStartup() error {
	if strings.TrimSpace(a.session.PrivateID) != "" {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	id, err := promptPrivateID("")
	if err != nil {
		return err
	}
	a.session.PrivateID = strings.TrimSpace(id)
	if err := a.saveSession(); err != nil {
		return err
	}
	fmt.Println("Private ID saved in session.")
	return nil
}

func (a *App) cmdOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	privateID := fs.String("private", a.session.PrivateID, "Private ID")
	creator := fs.String("creator", a.session.CreatorPublic, "Creator public ID")
	configPath := fs.String("config", a.session.StoreConfig, "Store config path")
	dbPath := fs.String("db", a.session.VaultDBPath, "Vault DB path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedPrivateID, err := promptPrivateID(*privateID)
	if err != nil {
		return err
	}
	a.session.PrivateID = strings.TrimSpace(resolvedPrivateID)
	if err := a.saveSession(); err != nil {
		return err
	}
	resolvedCreator, err := promptCreatorPublicID(*creator)
	if err != nil {
		return err
	}
	resolvedConfigPath, err := promptConfigPath(*configPath)
	if err != nil {
		return err
	}
	cfg, err := parseStoreConfig(resolvedConfigPath)
	if err != nil {
		return fmt.Errorf("cannot parse store config: %w", err)
	}
	a.closeAll()
	s, err := store.Open(cfg)
	if err != nil {
		return err
	}
	if err := ensureParentDir(*dbPath); err != nil {
		_ = s.Close()
		return err
	}
	db, err := sqlx.Open("sqlite3", *dbPath, "")
	if err != nil {
		_ = s.Close()
		return err
	}
	v, err := vault.Open(security.PrivateID(strings.TrimSpace(resolvedPrivateID)), security.PublicID(strings.TrimSpace(resolvedCreator)), s, db)
	if err != nil {
		_ = db.Close()
		_ = s.Close()
		return err
	}
	access, err := v.GetAccess(v.UserID)
	if err != nil {
		_ = v.Close()
		_ = db.Close()
		_ = s.Close()
		return err
	}
	a.store = s
	a.db = db
	a.v = v
	a.cwd = ""
	a.session.PrivateID = strings.TrimSpace(resolvedPrivateID)
	a.session.CreatorPublic = strings.TrimSpace(resolvedCreator)
	a.session.StoreConfig = strings.TrimSpace(resolvedConfigPath)
	a.session.VaultDBPath = strings.TrimSpace(*dbPath)
	_ = a.saveSession()
	fmt.Printf("Opened vault %s as %s (access=%s)\n", v.ID, v.UserID, access.String())
	return nil
}

func (a *App) cmdSync(args []string) error {
	if err := a.mustVault(); err != nil {
		return err
	}
	files, err := a.v.Sync()
	if err != nil {
		return err
	}
	fmt.Printf("Sync completed: %d new/updated files\n", len(files))
	return nil
}

func (a *App) cmdLs(args []string) error {
	if err := a.mustVault(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	limit := fs.Int("limit", 200, "Max entries")
	showAttrs := fs.Bool("attrs", false, "Show attrs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dir := a.cwd
	rest := fs.Args()
	if len(rest) > 0 {
		if strings.HasPrefix(rest[0], "/") {
			dir = normalizeVaultPath(rest[0])
		} else {
			dir = joinVaultPath(a.cwd, rest[0])
		}
	}
	files, err := a.v.ReadDir(dir, time.Time{}, 0, *limit)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tNAME\tSIZE\tMODTIME\tATTRS")
	for _, f := range files {
		typ := "FILE"
		if f.IsDir {
			typ = "DIR"
		}
		attrs := ""
		if *showAttrs {
			attrs = attrsPreview(f.Attrs)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", typ, f.Name, f.Size, f.ModTime.Format(time.RFC3339), attrs)
	}
	_ = tw.Flush()
	fmt.Printf("%d entries\n", len(files))
	return nil
}

func (a *App) cmdCd(args []string) error {
	if len(args) == 0 {
		a.cwd = ""
		fmt.Println("/")
		return nil
	}
	arg := strings.TrimSpace(args[0])
	if arg == ".." {
		if a.cwd == "" {
			fmt.Println("/")
			return nil
		}
		if i := strings.LastIndex(a.cwd, "/"); i >= 0 {
			a.cwd = a.cwd[:i]
		} else {
			a.cwd = ""
		}
		fmt.Println("/" + a.cwd)
		return nil
	}
	if strings.HasPrefix(arg, "/") {
		a.cwd = normalizeVaultPath(arg)
	} else {
		a.cwd = joinVaultPath(a.cwd, arg)
	}
	fmt.Println("/" + a.cwd)
	return nil
}

func (a *App) cmdPwd(args []string) error {
	fmt.Println("/" + a.cwd)
	return nil
}

func (a *App) cmdGet(args []string) error {
	if err := a.mustVault(); err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: get <remote-path> [local-path]")
	}
	remote := args[0]
	if strings.HasPrefix(remote, "/") {
		remote = normalizeVaultPath(remote)
	} else {
		remote = joinVaultPath(a.cwd, remote)
	}
	local := ""
	if len(args) > 1 {
		local = args[1]
	} else {
		local = filepath.Base(remote)
	}
	if err := ensureParentDir(local); err != nil {
		return err
	}
	f, err := a.v.Read(remote, local, vault.IOOption{}, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Downloaded %s -> %s (%d bytes)\n", remote, local, f.Size)
	return nil
}

func (a *App) cmdPut(args []string) error {
	if err := a.mustVault(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("put", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	attrs := fs.String("attrs", "", "Text attrs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("usage: put [--attrs text] <local-path> [remote-path]")
	}
	local := rest[0]
	remote := ""
	if len(rest) > 1 {
		remote = rest[1]
	} else {
		remote = filepath.Base(local)
	}
	if strings.HasPrefix(remote, "/") {
		remote = normalizeVaultPath(remote)
	} else {
		remote = joinVaultPath(a.cwd, remote)
	}
	f, err := a.v.Write(remote, local, []byte(*attrs), vault.IOOption{})
	if err != nil {
		return err
	}
	fmt.Printf("Uploaded %s -> %s (%d bytes)\n", local, f.Name, f.Size)
	return nil
}

func normalizeReplicaQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	if strings.HasPrefix(strings.ToUpper(q), "SQL:") {
		return "SQL:" + strings.TrimSpace(q[4:])
	}
	if strings.ContainsAny(q, " \t\n") {
		return "SQL:" + q
	}
	return q
}

func (a *App) cmdReplicaOpen(args []string) error {
	if err := a.mustVault(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("replica-open", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", a.session.ReplicaDBPath, "Replica DB path")
	dir := fs.String("dir", a.session.ReplicaDir, "Replica dir")
	ddlPath := fs.String("ddl", a.session.ReplicaDDLPath, "DDL file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir != "replica" {
		return fmt.Errorf("custom replica dir is not supported yet; use --dir replica")
	}
	ddl := ""
	if strings.TrimSpace(*ddlPath) != "" {
		b, err := os.ReadFile(*ddlPath)
		if err != nil {
			return err
		}
		ddl = string(b)
	}
	if a.replicaDB != nil {
		_ = a.replicaDB.Close()
		a.replicaDB = nil
		a.replica = nil
	}
	if err := ensureParentDir(*dbPath); err != nil {
		return err
	}
	rdb, err := sqlx.Open("sqlite3", *dbPath, ddl)
	if err != nil {
		return err
	}
	r, err := replica.Open(a.v, rdb)
	if err != nil {
		_ = rdb.Close()
		return err
	}
	a.replicaDB = rdb
	a.replica = r
	a.session.ReplicaDBPath = strings.TrimSpace(*dbPath)
	a.session.ReplicaDir = strings.TrimSpace(*dir)
	a.session.ReplicaDDLPath = strings.TrimSpace(*ddlPath)
	_ = a.saveSession()
	fmt.Printf("Replica opened: db=%s dir=%s\n", *dbPath, *dir)
	return nil
}

func (a *App) cmdReplicaSync(args []string) error {
	if err := a.mustReplica(); err != nil {
		return err
	}
	n, err := a.replica.Sync()
	if err != nil {
		return err
	}
	fmt.Printf("Replica sync done: %d updates\n", n)
	return nil
}

func (a *App) printRows(rows [][]any) {
	if len(rows) == 0 {
		fmt.Println("0 rows")
		return
	}
	maxCols := 0
	for _, r := range rows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for i := 0; i < maxCols; i++ {
		fmt.Fprintf(tw, "c%d\t", i+1)
	}
	fmt.Fprintln(tw)
	for _, r := range rows {
		for i := 0; i < maxCols; i++ {
			if i >= len(r) || r[i] == nil {
				fmt.Fprint(tw, "NULL\t")
				continue
			}
			switch v := r[i].(type) {
			case []byte:
				fmt.Fprintf(tw, "%s\t", attrsPreview(v))
			default:
				fmt.Fprintf(tw, "%v\t", v)
			}
		}
		fmt.Fprintln(tw)
	}
	_ = tw.Flush()
	fmt.Printf("%d rows\n", len(rows))
}

func parseJSONArgs(s string) (sqlx.Args, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return sqlx.Args{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return sqlx.Args(m), nil
}

func (a *App) cmdReplicaTables(args []string) error {
	if err := a.mustReplica(); err != nil {
		return err
	}
	rows, err := a.replica.Fetch("SQL:SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name", sqlx.Args{}, 1000)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) > 0 {
			fmt.Println(row[0])
		}
	}
	fmt.Printf("%d tables\n", len(rows))
	return nil
}

func (a *App) cmdReplicaQuery(args []string) error {
	if err := a.mustReplica(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("replica-query", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	max := fs.Int("max", 100, "Max rows")
	argsJSON := fs.String("args", "", "JSON args map")
	if err := fs.Parse(args); err != nil {
		return err
	}
	q := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if q == "" {
		return fmt.Errorf("usage: replica-query [--max N] [--args '{}'] <sql-or-key>")
	}
	m, err := parseJSONArgs(*argsJSON)
	if err != nil {
		return err
	}
	rows, err := a.replica.Fetch(normalizeReplicaQuery(q), m, *max)
	if err != nil {
		return err
	}
	a.printRows(rows)
	return nil
}

func (a *App) cmdReplicaExec(args []string) error {
	if err := a.mustReplica(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("replica-exec", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	argsJSON := fs.String("args", "", "JSON args map")
	if err := fs.Parse(args); err != nil {
		return err
	}
	q := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if q == "" {
		return fmt.Errorf("usage: replica-exec [--args '{}'] <sql-or-key>")
	}
	m, err := parseJSONArgs(*argsJSON)
	if err != nil {
		return err
	}
	res, err := a.replica.Exec(normalizeReplicaQuery(q), m)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	li, _ := res.LastInsertId()
	fmt.Printf("Exec OK rows=%d lastInsertId=%d\n", ra, li)
	return nil
}

func (a *App) cmdReplicaPreview(args []string) error {
	if err := a.mustReplica(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("replica-preview", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	limit := fs.Int("limit", 50, "row limit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) == 0 {
		return fmt.Errorf("usage: replica-preview [--limit N] <table>")
	}
	tableName := fs.Args()[0]
	q := fmt.Sprintf("SQL:SELECT * FROM \"%s\" LIMIT %d", tableName, *limit)
	rows, err := a.replica.Fetch(q, sqlx.Args{}, *limit)
	if err != nil {
		return err
	}
	a.printRows(rows)
	return nil
}

func (a *App) cmdStatus(args []string) error {
	fmt.Printf("Session: %s\n", a.sessionPath)
	fmt.Printf("Vault open: %t\n", a.v != nil)
	if a.v != nil {
		acc, _ := a.v.GetAccess(a.v.UserID)
		fmt.Printf("  id=%s\n  user=%s\n  author=%s\n  access=%s\n", a.v.ID, a.v.UserID, a.v.Author, acc.String())
	}
	fmt.Printf("Replica open: %t\n", a.replica != nil)
	fmt.Printf("cwd: /%s\n", a.cwd)
	return nil
}

func (a *App) cmdIdNew(args []string) error {
	id, err := security.NewPrivateID()
	if err != nil {
		return err
	}
	pub, err := id.PublicID()
	if err != nil {
		return err
	}
	fmt.Printf("private: %s\npublic:  %s\n", id, pub)
	return nil
}

func (a *App) cmdIdPublic(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: id-public <private-id>")
	}
	pub, err := security.PrivateID(strings.TrimSpace(args[0])).PublicID()
	if err != nil {
		return err
	}
	fmt.Println(pub)
	return nil
}

func (a *App) help() {
	fmt.Print(`Commands:
  help
  exit | quit
  status
  close

  id-new
  id-public <private-id>

  open --private <id> --creator <public-id> --config <store.yaml|json> [--db myapp/main.sqlite]
  sync
  ls [--limit N] [--attrs] [dir]
  cd [dir|..]
  up
  pwd
  get <remote-path> [local-path]
  put [--attrs text] <local-path> [remote-path]

  replica-open [--db myapp/replica.sqlite] [--dir replica] [--ddl queries.sql]
  replica-sync
  replica-tables
  replica-query [--max N] [--args '{"k":1}'] <sql-or-key>
  replica-exec  [--args '{"k":1}'] <sql-or-key>
  replica-preview [--limit N] <table>

  tui   Start full-screen text UI

Notes:
  - Paths without leading / are relative to current vault dir.
  - Session defaults are stored in ~/.bao/cli-session.yaml
`)
}

func (a *App) execute(parts []string) error {
	if len(parts) == 0 {
		return nil
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]
	switch cmd {
	case "help", "h", "?":
		a.help()
		return nil
	case "status":
		return a.cmdStatus(args)
	case "close":
		a.closeAll()
		fmt.Println("closed")
		return nil
	case "id-new":
		return a.cmdIdNew(args)
	case "id-public":
		return a.cmdIdPublic(args)
	case "open":
		return a.cmdOpen(args)
	case "sync":
		return a.cmdSync(args)
	case "ls", "dir":
		return a.cmdLs(args)
	case "cd":
		return a.cmdCd(args)
	case "up":
		return a.cmdCd([]string{".."})
	case "pwd":
		return a.cmdPwd(args)
	case "get", "read", "download":
		return a.cmdGet(args)
	case "put", "write", "upload":
		return a.cmdPut(args)
	case "replica-open":
		return a.cmdReplicaOpen(args)
	case "replica-sync":
		return a.cmdReplicaSync(args)
	case "replica-tables":
		return a.cmdReplicaTables(args)
	case "replica-query":
		return a.cmdReplicaQuery(args)
	case "replica-exec":
		return a.cmdReplicaExec(args)
	case "replica-preview":
		return a.cmdReplicaPreview(args)
	case "tui":
		return a.runTUI()
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}
