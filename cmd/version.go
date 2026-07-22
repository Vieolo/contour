package cmd

import (
	"bytes"

	"github.com/spf13/cobra"
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
		doc, _ := godotyaml.Parse(bytes.NewReader(ThisGyByte))
		termange.PrintInfof("v%s\n", doc.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
