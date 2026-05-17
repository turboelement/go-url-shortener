package osexit

import "os"

func main() {
	os.Exit(1) // want "call to os.Exit outside main function is not allowed"
}

func helper() {
	os.Exit(2) // want "call to os.Exit outside main function is not allowed"
}
