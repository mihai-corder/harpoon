// Command harpoon runs API request collections from the terminal.
//
// Collections are plain YAML files committed to your repo: debug requests
// interactively, chain variables between calls, and reuse the same files as
// smoke tests in CI.
package main

import "github.com/mihai-corder/harpoon/internal/cli"

func main() {
	cli.Execute()
}
