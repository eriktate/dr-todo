package main

import (
	"fmt"
	"log"

	todo "github.com/eriktate/dr-todo"
)

func main() {
	dt := todo.New("/home/soggy/.todos")
	name, err := dt.CreateToday()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s created ✅", name)
}
