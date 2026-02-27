package main

import (
	"fmt"
	"os"

	"github.com/AugustDG/ghotto/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gho:", err)
		os.Exit(1)
	}
}
