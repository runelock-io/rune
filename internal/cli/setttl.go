package cli

import (
	"fmt"
	"rune/internal/config"
	"rune/pkg/client"

	"github.com/spf13/cobra"
)

var setTTLCmd = &cobra.Command{
	Use:   "set-ttl [key] [ttl]",
	Short: "Set auto-rotation TTL on a secret (e.g. 7d, 24h, 30d). Pass empty string to clear.",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		token, err := config.LoadToken()
		if err != nil {
			Error("[AUTHENTICATION FAILED] Please login first using: rune login <token>")
			return
		}

		key := args[0]
		ttl := ""
		if len(args) == 2 {
			ttl = args[1]
		}

		c := client.New(serverURL(), token)
		ns := normalizeNamespace(namespaceName)

		if err := c.SetTTL(key, ns, ttl); err != nil {
			Error(err.Error())
			return
		}

		if ttl == "" {
			Success(fmt.Sprintf("TTL cleared for %q — auto-rotation disabled", key))
		} else {
			Success(fmt.Sprintf("TTL set to %s for %q — auto-rotation enabled", ttl, key))
		}
	},
}

func init() {
	setTTLCmd.Flags().StringVarP(&namespaceName, "namespace", "n", "default", "Namespace of the secret")
	rootCmd.AddCommand(setTTLCmd)
}
