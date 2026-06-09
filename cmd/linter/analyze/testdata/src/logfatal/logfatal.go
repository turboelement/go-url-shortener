package logfatal

import "log"

func main() {
	log.Fatal("error in main") // want "call to log.Fatal outside main function is not allowed"
}

func helper() {
	log.Fatal("error in helper") // want "call to log.Fatal outside main function is not allowed"
}
