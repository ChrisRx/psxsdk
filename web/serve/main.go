// Command serve serves a directory over HTTP on localhost, to try the site
// built into docs/ (task site:serve) as GitHub Pages would serve it.
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "localhost:8000", "address to listen on")
	dir := flag.String("dir", "docs", "directory to serve")
	flag.Parse()

	log.Printf("serving %s on http://%s/", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir))))
}
