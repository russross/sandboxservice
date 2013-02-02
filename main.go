package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ProblemType struct {
	Name      string
	Tag       string
	FieldList []ProblemField
}

type ProblemField struct {
	Name    string
	Prompt  string
	Title   string
	Type    string
	List    bool
	Default string
	Creator string
	Student string
	Grader  string
	Result  string
}

const (
	DefaultAddress       = ":80"
	CompressionThreshold = 1024
	Python27Name         = "bin/python2.7-static"
	SandboxName          = "bin/sandbox"
	MaxMB                = 256
	MaxSeconds           = 60
)

var Python27Path string
var SandboxPath string

func main() {
	if len(os.Args) > 2 {
		log.Fatalf("Usage: %s [[address]:port]", os.Args[0])
	}
	address := DefaultAddress
	if len(os.Args) == 2 {
		address = os.Args[1]
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to find working directory: %v", err)
	}
	Python27Path = filepath.Join(wd, Python27Name)
	SandboxPath = filepath.Join(wd, SandboxName)

	http.Handle("/python27stdin", jsonHandler(python27stdin_handler))
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL)
		writeJson(w, r, []*ProblemType{Python27StdinDescription})
	})

	log.Printf("Listening on %s", address)
	if err = http.ListenAndServe(address, nil); err != nil {
		log.Fatal(err)
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

type jsonHandler func(http.ResponseWriter, *http.Request, *json.Decoder)

func (h jsonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL)
	if r.Method != "POST" {
		log.Printf("JSON request called with method %s", r.Method)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		log.Printf("JSON request called with Content-Type %s", r.Header.Get("Content-Type"))
		http.Error(w, "Request must be in JSON format; must include Content-Type: appluication/json in request", http.StatusBadRequest)
		return
	}
	if !strings.Contains(r.Header.Get("Accept"), "application/json") {
		log.Printf("Accept is %s", r.Header.Get("Accept"))
		http.Error(w, "Client does not accept JSON response; must include Accept: application/json in request", http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	h(w, r, decoder)
}

func writeJson(w http.ResponseWriter, r *http.Request, elt interface{}) {
	raw, err := json.MarshalIndent(elt, "", "    ")
	if err != nil {
		log.Printf("Error encoding result as JSON: %v", err)
		http.Error(w, "Failure encoding result as JSON", http.StatusInternalServerError)
		return
	}
	size, actual := 0, 0
	if len(raw) > CompressionThreshold && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write(raw)
		gz.Close()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
		size = len(buf.Bytes())
		actual, err = w.Write(buf.Bytes())
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
		size = len(raw)
		actual, err = w.Write(raw)
	}
	if err != nil {
		log.Printf("Error writing result: %v", err)
		http.Error(w, "Failure writing JSON result", http.StatusInternalServerError)
	} else if size != actual {
		log.Printf("Output truncated")
		http.Error(w, "Output truncated", http.StatusInternalServerError)
	}
}
