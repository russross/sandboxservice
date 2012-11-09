package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type jsonHandler func(http.ResponseWriter, *http.Request, *json.Decoder)

func (h jsonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func grade_python27_inputoutput(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	in := new(Request)
	if err := decoder.Decode(in); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding input: %v", err), http.StatusBadRequest)
		return
	}

	result := new(InputOutputTestResult)
	writeJson(w, r, result)
}
