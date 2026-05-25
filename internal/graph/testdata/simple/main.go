package main

import (
	"fmt"
	"os"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func main() {
	r := graph.NewRegistry()
	_ = r
	fmt.Println("hello")
}

func helper(s string) string {
	return fmt.Sprintf("value: %s", s)
}

func run() error {
	if err := os.MkdirAll("/tmp/x", 0o755); err != nil {
		return err
	}
	result := helper("test")
	fmt.Println(result)
	return nil
}
