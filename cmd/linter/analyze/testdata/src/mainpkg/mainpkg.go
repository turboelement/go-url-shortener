package main

import (
	"log"
	"os"
)

func main() {
	log.Fatal("error in main") // ok
	os.Exit(1)                 // ok
}

func helper() {
	log.Fatal("error in helper") // want "call to log.Fatal outside main function is not allowed"
	os.Exit(2)                   // want "call to os.Exit outside main function is not allowed"
}
