package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	listen := flag.String("listen", ":8082", "listen address")
	flag.Parse()

	http.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		email := req.Header.Get("X-Portier-Email")

		rw.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		rw.Write([]byte(fmt.Sprintf("Logged in with: %s", email)))
	})

	log.Fatal(http.ListenAndServe(*listen, nil))
}
