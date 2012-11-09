package main

import (
	"bytes"
	"compress/gzip"
	"log"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"strconv"
)

type InputOutputTest string

type TestResult struct {
	Killed bool
	ExitStatus int
	Message string
	Stdout string
	Return string
	Stderr string
}
	
type InputOutputTestResult struct {
	Success bool

	CPUSeconds float64
	TotalSeconds float64
	MB float64

	Reference *TestResult
	Source *TestResult
}

type Request struct {
	MaxCPUSeconds float64
	MaxTotalSeconds float64
	MaxMB float64

	Reference string
	Source string

	Tests []*InputOutputTest
}

const PORT = ":8080"

func main() {
	http.Handle("/grade/python27/inputoutput", jsonHandler(grade_python27_inputoutput))

	log.Printf("Listening on %s", PORT)
	err := http.ListenAndServe(PORT, nil)
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

func writeJson(w http.ResponseWriter, r *http.Request, elt interface{}) {
    raw, err := json.Marshal(elt)
    if err != nil {
        log.Printf("Error encoding result as JSON: %v", err)
        http.Error(w, "Failure encoding result as JSON", http.StatusInternalServerError)
        return
    }
    size, actual := 0, 0
    if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
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
