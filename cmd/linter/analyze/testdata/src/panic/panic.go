package panic

func main() {
	panic("something went wrong in main") // want "panic use is not allowed"
}

func helper() {
	panic("something went wrong in helper") // want "panic use is not allowed"
}
