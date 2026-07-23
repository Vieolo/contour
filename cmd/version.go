package cmd

import (
	"bytes"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/godotyaml"
	"github.com/vieolo/termange"
)

// ThisGyByte holds the bytes of the project's go.yaml, embedded by main.go.
// It is the single source of truth for the CLI version.
var ThisGyByte []byte

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the version of contour",
	Long:  "Displays the version of contour",
	Run: func(cmd *cobra.Command, args []string) {
		termange.PrintInfof("v%s\n", cliVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// cliVersion returns the version from the embedded go.yaml, marked for
// development builds. The MCP server reports the same value to its clients.
func cliVersion() string {
	doc, _ := godotyaml.Parse(bytes.NewReader(ThisGyByte))

	v := doc.Version()
	if config.Dev {
		return v + "+dev"
	}
	return v
}
