package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/alexedwards/argon2id"
	"golang.org/x/term"
)

func main() {
	email := flag.String("email", "", "user email address (required)")
	flag.Parse()

	if *email == "" {
		fmt.Fprintln(os.Stderr, "usage: createuser -email <email>")
		os.Exit(1)
	}

	password, err := readMaskedPassword("Password: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading password: %v\n", err)
		os.Exit(1)
	}
	if password == "" {
		fmt.Fprintln(os.Stderr, "error: password cannot be empty")
		os.Exit(1)
	}

	confirm, err := readMaskedPassword("Confirm password: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading password: %v\n", err)
		os.Exit(1)
	}
	if password != confirm {
		fmt.Fprintln(os.Stderr, "error: passwords do not match")
		os.Exit(1)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./ev-charge.db"
	}

	db, err := database.Init(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)

	existing, err := userRepo.FindByEmail(ctx, *email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		os.Exit(1)
	}
	if existing != nil {
		fmt.Fprintf(os.Stderr, "error: email already registered\n")
		os.Exit(1)
	}

	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to hash password: %v\n", err)
		os.Exit(1)
	}

	user := &models.User{Email: *email, PasswordHash: hash}
	if err := userRepo.Create(ctx, user); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created user: %s (id=%s)\n", user.Email, user.ID)
}

// readMaskedPassword prompts for a password and displays * for each character typed.
// Backspace erases the last character. Ctrl+C aborts.
func readMaskedPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	var buf [1]byte
	var password []byte

	for {
		if _, err := os.Stdin.Read(buf[:]); err != nil {
			return "", err
		}
		b := buf[0]
		switch b {
		case '\r', '\n':
			fmt.Println()
			return string(password), nil
		case '\x03': // Ctrl+C
			fmt.Println()
			return "", fmt.Errorf("interrupted")
		case '\x7f', '\x08': // DEL / Backspace
			if len(password) > 0 {
				password = password[:len(password)-1]
				fmt.Print("\b \b")
			}
		default:
			if b >= 0x20 { // printable ASCII
				password = append(password, b)
				fmt.Print("*")
			}
		}
	}
}
