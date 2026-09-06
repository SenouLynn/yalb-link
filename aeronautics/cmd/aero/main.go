// Command aero prints the calculator version and exits.
package main

import (
	"fmt"
	"os"

	"yalb.aero/calculator"
)

func main() {
	if _, err := fmt.Fprintln(os.Stdout, "yalb.aero "+calculator.Version); err != nil {
		os.Exit(1)
	}
}
