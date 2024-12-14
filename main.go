package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/client"
)

var (
	Version = "0.0.1"
)

var (
	check   = flag.NewFlagSet("check", flag.ExitOnError)
	version = flag.NewFlagSet("version", flag.ExitOnError)
)

var commands = map[string]*flag.FlagSet{
	check.Name():   check,
	version.Name(): version,
}

func main() {

	if len(os.Args) == 1 {
		log.Fatalf("[ERROR] no command specified.")
	}

	cmd := commands[os.Args[1]]

	if cmd == nil {
		log.Fatalf("[ERROR] unknown subcommand '%s', see help for more details.", os.Args[1])
	}

	cmd.Parse(os.Args[2:])

	switch cmd.Name() {
	case "version":
		fmt.Println(Version)
		os.Exit(0)
	case "check":
		fmt.Println("[WPX] Checking development environment...")
		checkHasPHP()
		checkHasGit()
		checkHasDocker()
		os.Exit(0)
	}

	cmd.Usage()
}

func checkHasPHP() {
	fmt.Print("[WPX] ")
	cmd := exec.Command("php", "-v")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("PHP missing")
		os.Exit(1)
	}
	fmt.Print(strings.ToUpper(string(output)))
}

func checkHasGit() {
	fmt.Print("[WPX] ")
	cmd := exec.Command("git", "version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Git missing")
		os.Exit(1)
	}
	fmt.Print(strings.ToUpper(string(output)))
}

func checkHasDocker() {
	fmt.Print("[WPX] ")
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Println("Docker missing")
		os.Exit(1)
	}
	defer apiClient.Close()

	fmt.Printf("DOCKER VERSION: %s\n", apiClient.ClientVersion())
}
