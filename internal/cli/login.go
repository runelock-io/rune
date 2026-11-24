package cli

import (
	"fmt"
	"rune/internal/config"
	"rune/internal/constants"

	"github.com/spf13/cobra"
)

var loginServerURL string

var loginCmd = &cobra.Command{
	Use:   "login [token]",
	Short: "Authenticate with the Rune server and save your access token locally",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		token := args[0]

		if loginServerURL != "" {
			if err := config.SaveServerURL(loginServerURL); err != nil {
				Error("Failed to save server URL")
				return
			}
		}

		if err := config.SaveToken(token); err != nil {
			Error("Failed to save token")
			return
		}

		url := loginServerURL
		if url == "" {
			url = config.GetServerURL(constants.DefaultPort)
		}
		Success(fmt.Sprintf("Logged in — server: %s", url))
	},
}

func init() {
	loginCmd.Flags().StringVarP(&loginServerURL, "server", "s", "", "Rune server URL (e.g. http://1.2.3.4:2710)")
	rootCmd.AddCommand(loginCmd)
}
