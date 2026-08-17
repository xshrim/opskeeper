package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
	"opskeeper/backend/config"
	"opskeeper/backend/identity"
)

const (
	defaultBootstrapUsername    = "admin"
	generatedPasswordByteLength = 24
)

type createOptions struct {
	username     string
	email        string
	phone        string
	displayName  string
	password     string
	passwordFile string
	ifNeeded     bool
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "create" {
		fmt.Fprintln(os.Stderr, "usage: opskeeper-admin create [options]")
		os.Exit(2)
	}
	options, err := parseCreateOptions(os.Args[2:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		os.Exit(1)
	}
	username := firstNonBlank(options.username, os.Getenv("OPSK_BOOTSTRAP_USERNAME"), defaultBootstrapUsername)
	email := firstNonBlank(options.email, os.Getenv("OPSK_BOOTSTRAP_EMAIL"))
	phone := firstNonBlank(options.phone, os.Getenv("OPSK_BOOTSTRAP_PHONE"))
	displayName := firstNonBlank(options.displayName, os.Getenv("OPSK_BOOTSTRAP_DISPLAY_NAME"), username)
	passwordFile := firstNonBlank(options.passwordFile, os.Getenv("OPSK_BOOTSTRAP_PASSWORD_FILE"))
	password, generated, err := loadPassword(options.password, passwordFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read password: %v\n", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure PostgreSQL client: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()
	ctx := context.Background()
	identityService := identity.NewService(identity.NewStore(pool), cfg.SessionAccessTTL, cfg.SessionRefreshTTL)
	authorizationService := authorization.NewService(authorization.NewStore(pool))
	admin, err := identityService.BootstrapAdmin(ctx, identity.BootstrapInput{Username: username, Email: email, Phone: phone, DisplayName: displayName, Password: password})
	if err != nil {
		if options.ifNeeded && errors.Is(err, identity.ErrBootstrapComplete) {
			fmt.Fprintln(os.Stdout, "bootstrap administrator already exists; skipping")
			return
		}
		fmt.Fprintf(os.Stderr, "create bootstrap administrator: %v\n", err)
		os.Exit(1)
	}
	if generated {
		fmt.Fprintf(os.Stdout, "generated bootstrap credentials (store securely):\nusername: %s\npassword: %s\n", username, password)
	}
	if err := authorizationService.EnsureBootstrapAdmin(ctx, admin.ID); err != nil {
		fmt.Fprintf(os.Stderr, "bind bootstrap administrator role: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "bootstrap administrator created")
}

func parseCreateOptions(args []string, output io.Writer) (createOptions, error) {
	options := createOptions{}
	flags := flag.NewFlagSet("opskeeper-admin create", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.username, "username", "", "bootstrap username")
	flags.StringVar(&options.email, "email", "", "bootstrap administrator email")
	flags.StringVar(&options.phone, "phone", "", "bootstrap administrator phone")
	flags.StringVar(&options.displayName, "display-name", "", "bootstrap administrator display name")
	flags.StringVar(&options.password, "password", "", "bootstrap password (visible in process arguments and shell history)")
	flags.StringVar(&options.passwordFile, "password-file", "", "path to a bootstrap password file")
	flags.BoolVar(&options.ifNeeded, "if-needed", false, "succeed without changes when a bootstrap user already exists")
	flags.Usage = func() {
		fmt.Fprintln(output, "usage: opskeeper-admin create [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return createOptions{}, err
	}
	if flags.NArg() != 0 {
		return createOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	return options, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func loadPassword(password, passwordFile string) (string, bool, error) {
	if strings.TrimSpace(password) != "" {
		return password, false, nil
	}
	if passwordFile != "" {
		bytes, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", false, err
		}
		password := strings.TrimRight(string(bytes), "\r\n")
		if password == "" {
			return "", false, errors.New("password file is empty")
		}
		return password, false, nil
	}

	password, err := generatePassword()
	if err != nil {
		return "", false, err
	}
	return password, true, nil
}

func generatePassword() (string, error) {
	bytes := make([]byte, generatedPasswordByteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
