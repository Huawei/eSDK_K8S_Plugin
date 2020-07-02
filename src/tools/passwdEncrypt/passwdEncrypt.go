package main

import (
	"fmt"
	"os"
	"utils/pwd"

	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	var plainText string

	if len(os.Args) >= 2 {
		plainText = os.Args[1]
	} else {
		fmt.Print("Enter password: ")
		input, err := terminal.ReadPassword(0)
		if err != nil {
			fmt.Println("Input password error: %v", err)
			os.Exit(1)
		}

		plainText = string(input)
		fmt.Println("")
	}

	encrypted, err := pwd.Encrypt(plainText)
	if err != nil {
		fmt.Println("Encrypt password error: %v", err)
		os.Exit(1)
	}

	fmt.Println("Encrypted password:", encrypted)
}
