// Command serve serves a directory over HTTP on localhost, to try the site
// built into .build/site (task site:serve) as GitHub Pages would serve it.
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "localhost:8000", "address to listen on")
	dir := flag.String("dir", ".build/site", "directory to serve")
	flag.Parse()

	// Browsers may keep a file for hours without asking again when nothing
	// says otherwise, and Firefox does so even on a reload for what a worker
	// fetches, so a rebuilt psx.wasm would not be seen. Have them ask.
	files := http.FileServer(http.Dir(*dir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})

	log.Printf("serving %s on http://%s/", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
