package main

import (
	"fmt"

	"github.com/kuangchanglang/graceful"
)

func main() {
	server := graceful.NewServer()
	server.Register("0.0.0.0:9222", nil)
	err := server.Run()
	fmt.Printf("error: %v\n", err)
}
