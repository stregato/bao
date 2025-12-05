//go:build !js && !ios
// +build !js,!ios

package main

import (
	"fmt"
)

func main() {
	fmt.Print("This is just a library! ")
}
