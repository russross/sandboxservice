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
	"time"
	"unicode"
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

type GenericResponse struct {
	Report string
	Passed bool
}

type TestResult struct {
	Error   bool
	Message string
	Stdout  string
	Stderr  string
}

const (
	DefaultAddress       = ":8081"
	CompressionThreshold = 1024
	Python27Name         = "/usr/local/bin/python2.7-static"
	SandboxName          = "/usr/local/bin/sandbox"
	LogFileName          = "/var/log/sandbox/sandboxservice.log"
	MaxMB                = 256
	MaxSeconds           = 60
	JSONIndent           = true
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

	// set log file
	logfile, err := os.OpenFile(LogFileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Failed to open logfile %s: %v", LogFileName, err)
	}
	defer logfile.Close()
	log.SetOutput(logfile)

	Python27Path = Python27Name
	SandboxPath = SandboxName

	http.Handle("/grade/python27stdin", jsonHandler(python27stdin_handler))
	http.Handle("/grade/python27module", jsonHandler(python27module_handler))
	http.Handle("/output/python27stdin", jsonHandler(python27stdin_output_handler))
	http.Handle("/output/python27module", jsonHandler(python27module_output_handler))
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL)
		writeJson(w, r, []*ProblemType{
			Python27StdinDescription,
			Python27ModuleDescription,
		})
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

func fixLineEndings(s string) string {
	s = strings.Replace(s, "\r\n", "\n", -1)
	if !strings.HasSuffix(s, "\n") {
		s = s + "\n"
	}
	for strings.Contains(s, " \n") {
		s = strings.Replace(s, " \n", "\n", -1)
	}
	return s
}

func isEmpty(s string) bool {
	for _, ch := range s {
		if !unicode.IsSpace(ch) {
			return false
		}
	}
	return true
}

type jsonHandler func(http.ResponseWriter, *http.Request, *json.Decoder)

func (h jsonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL)
	start := time.Now()
	if r.Method != "POST" {
		log.Printf("JSON request called with method %s", r.Method)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		log.Printf("JSON request called with Content-Type %s", r.Header.Get("Content-Type"))
		http.Error(w, "Request must be in JSON format; must include Content-Type: application/json in request", http.StatusBadRequest)
		return
	}
	if !strings.Contains(r.Header.Get("Accept"), "application/json") && !strings.Contains(r.Header.Get("Accept"), "*/*") {
		log.Printf("Client does not accept application/json; Accept is %s", r.Header.Get("Accept"))
		http.Error(w, "Client does not accept JSON response; must include Accept: application/json in request", http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	h(w, r, decoder)
	log.Printf("  request completed in %v", time.Since(start))
}

func writeJson(w http.ResponseWriter, r *http.Request, elt interface{}) {
	var raw []byte
	var err error
	if JSONIndent {
		raw, err = json.MarshalIndent(elt, "", "    ")
	} else {
		raw, err = json.Marshal(elt)
	}
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
