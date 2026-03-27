// main.go
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

var version = "dev"

type CLI struct {
	Dir     string `short:"C" help:"Path to git repository." default:"." type:"path"`
	Remote  string `short:"r" help:"Git remote to use." default:""`
	Version kong.VersionFlag `short:"v" help:"Print version."`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("manifold"),
		kong.Description("Monitor CI/CD pipelines from the terminal."),
		kong.Vars{"version": version},
	)

	fmt.Fprintf(os.Stderr, "manifold %s — dir=%s remote=%s\n", version, cli.Dir, cli.Remote)
}
