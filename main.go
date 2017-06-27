package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
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
var seed = flag.Int64("seed", 0, "random seed for song")

type RandReader struct {
	size int
	rnd  *rand.Rand
}

func NewRandReader(size int, seed int64) *RandReader {
	return &RandReader{
		size: size,
		rnd:  rand.New(rand.NewSource(seed)),
	}
}

func (rr *RandReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(rr.rnd.Int())
		rr.size--
		n++
		if rr.size == 0 {
			err = errors.New("End of random stream")
			return
		}
	}
	return
}

func inverse(buffer []byte) {
	length := len(buffer)
	input, major, minor := make([]byte, length), [256]int{}, make([]int, length)
	for k, v := range buffer {
		input[k], minor[k], major[v] = v, major[v], major[v]+1
	}

	sum := 0
	for k, v := range major {
		major[k], sum = sum, sum+v
	}

	j := length - 1
	for k, _ := range input {
		for minor[k] != -1 {
			buffer[j], j, k, minor[k] = input[k], j-1, major[input[k]]+minor[k], -1
		}
	}
}

type StructuredReader struct {
	*bytes.Reader
}

func NewStructuredReader(size int, seed int64) *StructuredReader {
	rnd := rand.New(rand.NewSource(seed))
	samples := make([]byte, 4)
	for i := range samples {
		samples[i] = byte(rnd.Int())
	}
	j, out := 0, make([]byte, size)
	for i := range out {
		out[i] = samples[j]
		j++
		if j >= len(samples) {
			inverse(samples)
			j = 0
		}
	}
	return &StructuredReader{bytes.NewReader(out)}
}

type HoloReader struct {
	*bytes.Reader
}

func NewHoloReader(size int, seed int64) *HoloReader {
	rnd := rand.New(rand.NewSource(seed))
	out := make([]byte, size)
	for i := range out {
		sample := math.Abs(rnd.NormFloat64() * 8)
		if sample > 256 {
			sample = 256
		}
		out[i] = byte(sample)
	}

	//move to front
	nodes := [256]byte{}
	var first byte

	for node, _ := range nodes {
		nodes[node] = uint8(node) + 1
	}

	for i, symbol := range out {
		var node, next byte
		moveToFront := symbol != 0
		for next = first; symbol > 0; node, next = next, nodes[next] {
			symbol--
		}

		if moveToFront {
			first, nodes[node], nodes[next] = next, nodes[next], first
		}

		out[i] = next
	}

	//burrows wheeler
	inverse(out)

	return &HoloReader{bytes.NewReader(out)}
}

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

	if *seed != 0 {
		in := NewStructuredReader(2*1024*1024, *seed)
		h := hasher.New(in)
		sng := h.Hash()

		wav := wave.New(wave.Stereo, 22000)
		ks := karplus.Song{
			Song:         *sng,
			SamplingRate: 22000,
		}
		ks.Sound(&wav)

		out, err := os.Create(fmt.Sprintf("%v.wav", *seed))
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
