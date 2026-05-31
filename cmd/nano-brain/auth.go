package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func runAuthCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain auth <hash|token>")
		os.Exit(1)
	}

	switch args[0] {
	case "hash":
		runAuthHash(args[1:])
	case "token":
		runAuthToken()
	default:
		fmt.Fprintf(os.Stderr, "Unknown auth subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runAuthHash(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain auth hash <password>")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(args[0]), 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}

func runAuthToken() {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("nbt_" + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b))
}
