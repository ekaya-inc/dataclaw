package main

import (
	"fmt"
	"os"

	"github.com/ekaya-inc/dataclaw/internal/app"
)

var Version = "dev"

func main() {
	if err := app.Run(Version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
