package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pointlander/chopher/api"
	"github.com/pointlander/chopher/hasher"
	"github.com/pointlander/chopher/karplus"
	"github.com/pointlander/chopher/wave"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var file = flag.String("file", "", "file to hash")

func main() {
	flag.Parse()

	if *file != "" {
		in, err := os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
		h := hasher.New(in)
		sng := h.Hash()
		in.Close()

		wav := wave.New(wave.Stereo, 22000)
		ks := karplus.Song{
			Song:         *sng,
			SamplingRate: 22000,
		}
		ks.Sound(&wav)

		out, err := os.Create(strings.TrimSuffix(*file, filepath.Ext(*file)) + ".wav")
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(out, wav.Reader())
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	r := mux.NewRouter()
	r.StrictSlash(true)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/"))).Methods("GET")
	r.HandleFunc("/upload", api.FileUploadHandler).Methods("POST")
	http.ListenAndServe(":"+port, handlers.LoggingHandler(os.Stdout, r))
}
