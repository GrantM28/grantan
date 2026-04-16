package main

import (
	"log"

	"imperials/grantan"
)

func main() {
	server := grantan.NewServer(grantan.LoadConfig())
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
