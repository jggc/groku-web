package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed web/*
var webFS embed.FS

const version = "0.5.0"

const usage = `usage: groku [--version] [--help] <command> [<args>]

CLI remote for your Roku

Commands:
  home            Return to the home screen
  rev             Reverse
  fwd             Fast Forward
  select          Select
  left            Left
  right           Right
  up              Up
  down            Down
  back            Back
  info            Info
  backspace       Backspace
  enter           Enter
  search          Search
  replay          Replay
  play            Play
  pause           Pause
  discover        Discover a roku on your local network
  text            Send text to the Roku
  apps            List installed apps on your Roku
  app             Launch specified app
  serve           Start web UI (default :8080)
`

func main() {
	store := newDeviceStore(fmt.Sprintf("%s/groku.json", os.TempDir()))

	if len(os.Args) == 1 || isHelp(os.Args[1]) {
		fmt.Print(usage)
		os.Exit(0)
	}
	if os.Args[1] == "-v" || os.Args[1] == "--version" {
		fmt.Printf("groku version %s\n", version)
		os.Exit(0)
	}

	cmd := os.Args[1]
	switch cmd {
	case "serve":
		runServe(store, os.Args[2:])
	case "home", "rev", "fwd", "select", "left", "right", "down", "up",
		"back", "info", "backspace", "enter", "search":
		mustKey(store, capitalize(cmd))
	case "replay":
		mustKey(store, "InstantReplay")
	case "play", "pause":
		mustKey(store, "Play")
	case "discover":
		loc, err := store.loadOrDiscover()
		if err != nil {
			fatal(err)
		}
		fmt.Println("Found roku at", loc)
	case "text":
		if len(os.Args) < 3 {
			fmt.Print(usage)
			os.Exit(1)
		}
		loc, err := store.ensure()
		if err != nil {
			fatal(err)
		}
		if err := sendText(loc, os.Args[2]); err != nil {
			fatal(err)
		}
	case "apps":
		loc, err := store.ensure()
		if err != nil {
			fatal(err)
		}
		apps, err := queryApps(loc)
		if err != nil {
			fatal(err)
		}
		for _, a := range apps {
			fmt.Println(a.Name)
		}
	case "app":
		if len(os.Args) < 3 {
			fmt.Print(usage)
			os.Exit(1)
		}
		loc, err := store.ensure()
		if err != nil {
			fatal(err)
		}
		apps, err := queryApps(loc)
		if err != nil {
			fatal(err)
		}
		name := strings.Join(os.Args[2:], " ")
		for _, a := range apps {
			if a.Name == name {
				if err := launchApp(loc, a.ID); err != nil {
					fatal(err)
				}
				return
			}
		}
		fmt.Printf("App %q not found\n", name)
		os.Exit(1)
	default:
		fmt.Print(usage)
		os.Exit(1)
	}
}

func runServe(store *deviceStore, args []string) {
	addr := ":8080"
	roku := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-addr", "--addr":
			i++
			if i < len(args) {
				addr = args[i]
			}
		case "-roku", "--roku":
			i++
			if i < len(args) {
				roku = args[i]
			}
		}
	}
	if roku != "" {
		store.set(roku)
	} else {
		if loc, err := store.loadOrDiscover(); err == nil {
			fmt.Println("using roku at", loc)
		} else {
			fmt.Println("no roku cached yet — pick one in the UI (r)")
		}
	}

	srv := &apiServer{store: store, web: webFS}
	fmt.Printf("groku web ui on http://127.0.0.1%s\n", addr)
	if err := http.ListenAndServe(addr, srv.routes()); err != nil {
		fatal(err)
	}
}

func mustKey(store *deviceStore, key string) {
	loc, err := store.ensure()
	if err != nil {
		// force rediscover if cache empty/stale
		locs, derr := discoverRokus(3 * time.Second)
		if derr != nil || len(locs) == 0 {
			fatal(err)
		}
		store.set(locs[0])
		loc = store.get()
	}
	if err := keypress(loc, key); err != nil {
		fatal(err)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func isHelp(s string) bool {
	switch s {
	case "--help", "-help", "--h", "-h", "help":
		return true
	}
	return false
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
