package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/yaoapp/registry/auth"
	"github.com/yaoapp/registry/config"
	"golang.org/x/term"
)

// RunUser executes the "user" subcommand with add/remove/passwd/list actions.
func RunUser(args []string) {
	if len(args) == 0 {
		printUserUsage()
		os.Exit(1)
	}

	action := args[0]
	rest := args[1:]

	switch action {
	case "add":
		runUserAdd(rest)
	case "remove":
		runUserRemove(rest)
	case "passwd":
		runUserPasswd(rest)
	case "list":
		runUserList(rest)
	default:
		fmt.Fprintf(os.Stderr, "Unknown user action: %s\n", action)
		printUserUsage()
		os.Exit(1)
	}
}

func printUserUsage() {
	fmt.Fprintln(os.Stderr, "Usage: registry user <action> [flags]")
	fmt.Fprintln(os.Stderr, "  add      Add a new user")
	fmt.Fprintln(os.Stderr, "  remove   Remove a user")
	fmt.Fprintln(os.Stderr, "  passwd   Change user password")
	fmt.Fprintln(os.Stderr, "  list     List all users")
}

func loadAuthFile(flagArgs []string) (*auth.AuthFile, *flag.FlagSet) {
	cfg := config.LoadFromEnv(nil)
	fs := flag.NewFlagSet("user", flag.ExitOnError)
	authFilePath := fs.String("auth-file", cfg.AuthFile, "Authentication file path")
	fs.Parse(flagArgs)
	af := auth.NewAuthFile(*authFilePath)
	if err := af.Load(); err != nil {
		log.Fatalf("load auth file: %v", err)
	}
	return af, fs
}

func runUserAdd(args []string) {
	fs := flag.NewFlagSet("user add", flag.ExitOnError)

	cfg := config.LoadFromEnv(nil)
	authFilePath := fs.String("auth-file", cfg.AuthFile, "Authentication file path")
	password := fs.String("password", "", "User password (interactive prompt if omitted)")
	fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Usage: registry user add <username> [--password <pass>]")
		os.Exit(1)
	}
	username := fs.Arg(0)

	pass := *password
	if pass == "" {
		pass = promptPassword("Password: ")
	}

	af := auth.NewAuthFile(*authFilePath)
	if err := af.Load(); err != nil {
		log.Fatalf("load auth file: %v", err)
	}

	if err := af.AddUser(username, pass); err != nil {
		log.Fatalf("add user: %v", err)
	}
	fmt.Printf("User %q added.\n", username)
}

func runUserRemove(args []string) {
	af, fs := loadAuthFile(args)
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Usage: registry user remove <username>")
		os.Exit(1)
	}
	username := fs.Arg(0)
	if err := af.RemoveUser(username); err != nil {
		log.Fatalf("remove user: %v", err)
	}
	fmt.Printf("User %q removed.\n", username)
}

func runUserPasswd(args []string) {
	fs := flag.NewFlagSet("user passwd", flag.ExitOnError)
	cfg := config.LoadFromEnv(nil)
	authFilePath := fs.String("auth-file", cfg.AuthFile, "Authentication file path")
	password := fs.String("password", "", "New password (interactive prompt if omitted)")
	fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Usage: registry user passwd <username> [--password <pass>]")
		os.Exit(1)
	}
	username := fs.Arg(0)

	pass := *password
	if pass == "" {
		pass = promptPassword("New password: ")
	}

	af := auth.NewAuthFile(*authFilePath)
	if err := af.Load(); err != nil {
		log.Fatalf("load auth file: %v", err)
	}

	if err := af.UpdatePassword(username, pass); err != nil {
		log.Fatalf("update password: %v", err)
	}
	fmt.Printf("Password updated for %q.\n", username)
}

func runUserList(args []string) {
	af, _ := loadAuthFile(args)
	users := af.ListUsers()
	if len(users) == 0 {
		fmt.Println("No users.")
		return
	}
	fmt.Println(strings.Join(users, "\n"))
}

func promptPassword(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatalf("read password: %v", err)
	}
	return string(pw)
}
