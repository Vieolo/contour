package main

import (
	_ "embed"

	"github.com/vieolo/contour/cmd"
)

//go:embed go.yaml
var thisGyByte []byte

func main() {
	cmd.ThisGyByte = thisGyByte
	cmd.Execute()
}
