package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
	"opskeeper/backend/config"
	"opskeeper/backend/identity"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "create" {
		fmt.Fprintln(os.Stderr, "usage: opskeeper-admin create")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		os.Exit(1)
	}
	input := bufio.NewReader(os.Stdin)
	email, err := inputValue("Email", os.Getenv("OPSK_BOOTSTRAP_EMAIL"), input, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read email: %v\n", err)
		os.Exit(1)
	}
	displayName, err := inputValue("Display name", os.Getenv("OPSK_BOOTSTRAP_DISPLAY_NAME"), input, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read display name: %v\n", err)
		os.Exit(1)
	}
	password, err := readPassword(os.Getenv("OPSK_BOOTSTRAP_PASSWORD_FILE"), os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read password: %v\n", err)
		os.Exit(1)
	}
	confirmation, err := readPassword(os.Getenv("OPSK_BOOTSTRAP_PASSWORD_FILE"), os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read password confirmation: %v\n", err)
		os.Exit(1)
	}
	if password != confirmation {
		fmt.Fprintln(os.Stderr, "passwords do not match")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure PostgreSQL client: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()
	service := identity.NewService(identity.NewStore(pool), cfg.SessionAccessTTL, cfg.SessionRefreshTTL)
	if _, err := service.BootstrapAdmin(context.Background(), identity.BootstrapInput{Email: email, DisplayName: displayName, Password: password}); err != nil {
		fmt.Fprintf(os.Stderr, "create bootstrap administrator: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "bootstrap administrator created")
}

func inputValue(label, value string, input *bufio.Reader, output io.Writer) (string, error) {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}
	if _, err := fmt.Fprintf(output, "%s: ", label); err != nil {
		return "", err
	}
	line, err := input.ReadString('\n')
	return strings.TrimSpace(line), err
}

func readPassword(passwordFile string, input *os.File, output io.Writer) (string, error) {
	if passwordFile != "" {
		bytes, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", err
		}
		password := strings.TrimRight(string(bytes), "\r\n")
		if password == "" {
			return "", errors.New("password file is empty")
		}
		return password, nil
	}
	if !term.IsTerminal(int(input.Fd())) {
		return "", errors.New("password input requires a terminal or OPSK_BOOTSTRAP_PASSWORD_FILE")
	}
	if _, err := fmt.Fprint(output, "Password: "); err != nil {
		return "", err
	}
	bytes, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(output)
	return string(bytes), err
}
