package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	zxcvbn "github.com/nbutton23/zxcvbn-go"

	"golang.org/x/crypto/ssh/terminal"
)

const (
	maxNewPasswordTries = 10
)

// PromptNewPassword prompts the user for a new vault password.
// The user is requird to type the password a second time for confirmation,
// and the password must meet minimum entropy.
// If unsure, a value of 50.0 is reasonable for a moderately-strong password.
func promptNewPassword(prompt string, minEntropy float64) (string, error) {
	for i := 0; i < maxNewPasswordTries; i++ {
		password, err := terminalGetSecret(prompt)
		if err != nil {
			return "", fmt.Errorf("Input error: %v", err)
		}
		if len(password) == 0 || zxcvbn.PasswordStrength(password, nil).Entropy < minEntropy {
			log.Printf("Password weak - please try again\n\n")
			continue
		}
		confirm, err := terminalGetSecret("Confirm password:")
		if err != nil {
			log.Printf("Input error: please try again\n\n")
			continue
		}
		if password != confirm {
			log.Printf("Passwords did not match - please try again\n\n")
			continue
		}
		return password, nil
	}
	return "", errors.New("Too many tries. Please try again later")
}

// promptPassword asks the user to enter a password.
// This can be used to ask for a password for an existing vault.
func promptPassword() (string, error) {
	password, err := terminalGetSecret("Enter vault password:")
	if err != nil {
		return "", fmt.Errorf("Input error: %v", err)
	}
	return password, nil
}

// terminalGetSecret - ask user for password or secret key.
// Typed entry is not echoed to terminal.
func terminalGetSecret(prompt string) (string, error) {

	fmt.Print(prompt)
	bytePassword, err := terminal.ReadPassword(0)
	fmt.Println()
	if err != nil {
		return "", err
	}
	password := string(bytePassword)
	return strings.TrimSpace(password), nil
}
