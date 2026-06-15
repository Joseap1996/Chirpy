package main

import (
	"fmt"
	"net/http"
)

func main() {
	serveMux := http.NewServeMux()
	serveStruct := http.Server{}
	serveStruct.Addr = ":8080"
	serveStruct.Handler = serveMux
	serveMux.Handle("/", http.FileServer(http.Dir(".")))

	if err := serveStruct.ListenAndServe(); err != nil {
		fmt.Println(err)
	}

}
