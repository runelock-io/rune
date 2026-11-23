package cli

import (
	"fmt"
	"os"
	"rune/internal/config"
	"rune/internal/constants"

	"github.com/spf13/cobra"
)

var showVersion bool

const (
	reset  = "\033[0m"
	dim    = "\033[2m"
	bold   = "\033[1m"
	accent = "\033[38;5;110m" // muted slate blue
	subtle = "\033[38;5;245m" // mid gray
	ok     = "\033[38;5;108m" // muted sage green
)

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func c(code, s string) string {
	if !colorEnabled() {
		return s
	}
	return code + s + reset
}

var rootCmd = &cobra.Command{
	Use:   "rune",
	Short: "A lightweight, secure secrets manager",
	Long:  "rune is a minimal, local-first secrets manager. Encrypted storage, sealed by default.",
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Printf("rune %s\n", Version)
			return
		}
		printBanner()
	},
}

func printBanner() {
	mark := []string{
		` _ __ _   _ _ __   ___ `,
		`| '__| | | | '_ \ / _ \`,
		`| |  | |_| | | | |  __/`,
		`|_|   \__,_|_| |_|\___|`,
	}
	for _, line := range mark {
		fmt.Println(c(accent, line))
	}
	fmt.Println()
	fmt.Println(c(bold, "rune") + c(subtle, " · local-first secrets, sealed by default"))
	fmt.Printf("%s\n\n", c(dim, Version))

	fmt.Println(c(subtle, "  status  ") + c(ok, "●") + " sealed")
	fmt.Println()

	fmt.Println(c(subtle, "USAGE"))
	fmt.Println("  rune <command> [flags]")
	fmt.Println()

	fmt.Println(c(subtle, "VAULT"))
	printRow("put", "store and encrypt a secret")
	printRow("get", "retrieve and decrypt a secret")
	printRow("list", "list stored keys")
	printRow("delete", "delete a secret")
	printRow("rotate", "generate a new value for a secret")
	printRow("history", "show a secret's version history")
	fmt.Println()

	fmt.Println(c(subtle, "ACCESS"))
	printRow("unseal", "unseal the vault with your passphrase")
	printRow("seal", "seal the vault, clearing the key from memory")
	printRow("status", "check whether the vault is sealed")
	fmt.Println()

	fmt.Println(c(subtle, "ORG"))
	printRow("namespace", "manage namespaces")
	printRow("login", "authenticate with the rune server")
	printRow("token", "manage authentication tokens")
	fmt.Println()

	fmt.Println(c(dim, "  rune <command> --help   details on a command"))
	fmt.Println(c(dim, "  rune help                full command list"))
}

func printRow(name, desc string) {
	fmt.Printf("  %s%-10s%s %s\n", accent, name, reset, c(subtle, desc))
}

func init() {
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version")
}

func serverURL() string {
	if env := os.Getenv("RUNE_SERVER"); env != "" {
		return env
	}
	return config.GetServerURL(constants.DefaultPort)
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
