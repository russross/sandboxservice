package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	Port                 = ":8080"
	CompressionThreshold = 1024
)

func main() {
	http.Handle("/grade/python27/inputoutput", jsonHandler(grade_python27_inputoutput))
	http.Handle("/grade/python27/expression", jsonHandler(grade_python27_expression))

	log.Printf("Listening on %s", Port)
	err := http.ListenAndServe(Port, nil)
	if err != nil {
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
	if r.Method != "POST" {
		log.Printf("method is %s", r.Method)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		log.Printf("Content-Type is %s", r.Header.Get("Content-Type"))
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
