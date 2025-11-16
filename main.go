package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args, os.Stdout, os.Stderr); err != nil {
		fmt.Println("error:")
		fmt.Println(err)
	}
	fmt.Println("done")
}

func run(ctx context.Context, args []string, output *os.File, outerror *os.File) error {
	return nil
}
