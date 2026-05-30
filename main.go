package main

/*
 * ░█▀▀░█░█░▀█▀░▀▀█░█░█░█░█░█▀█
 * ░▀▀█░█▀█░░█░░▄▀░░█░█░█▀▄░█▀█
 * ░▀▀▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀░▀░▀░▀
 */

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/olimci/shizuka/cmd"
)

func main() {
	defer func() {
		if v := recover(); v != nil {
			fmt.Fprintf(os.Stderr, "There was an unexpected error: %v\nPlease report this at https://github.com/olimci/shizuka/issues\n", v)
			os.Exit(1)
		}
	}()

	if err := cmd.Execute(context.Background(), os.Args); err != nil {
		if h, is := errors.AsType[*cmd.HandledError](err); is {
			os.Exit(h.Code)
		}

		fmt.Fprintf(os.Stderr, "There was an unexpected error: %v\nPlease report this at https://github.com/olimci/shizuka/issues\n", err)
		os.Exit(1)
	}
}
