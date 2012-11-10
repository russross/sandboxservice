package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	python27name = "bin/python2.7-static"
	sandboxname  = "bin/sandbox"
)

var python27path string
var sandboxpath string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic("Failed to find working directory: " + err.Error())
	}
	python27path = filepath.Join(wd, python27name)
	sandboxpath = filepath.Join(wd, sandboxname)
}

type TestResult struct {
	Error   bool
	Message string
	Stdout  string
	Stderr  string
}

type TestResultPair struct {
	Success   bool
	Reference *TestResult
	Candidate *TestResult
}

type Request struct {
	MaxCPUSeconds   int
	MaxTotalSeconds int
	MaxMB           int

	Reference string
	Candidate string

	Tests []string
}

func (elt *Request) Validate() error {
	if elt.MaxCPUSeconds < 1 {
		return fmt.Errorf("MaxCPUSeconds must be >= 1")
	} else if elt.MaxCPUSeconds > 60 {
		return fmt.Errorf("MaxCPUSeconds must be <= 60")
	}

	if elt.MaxTotalSeconds < 1 {
		return fmt.Errorf("MaxTotalSeconds must be >= 1")
	} else if elt.MaxTotalSeconds > 60 {
		return fmt.Errorf("MaxTotalSeconds must be <= 60")
	}

	if elt.MaxMB < 1 {
		return fmt.Errorf("MaxMB must be >= 1")
	} else if elt.MaxMB > 256 {
		return fmt.Errorf("MaxMB must be <= 256")
	}

	if elt.Reference == "" {
		return fmt.Errorf("Reference solution is required")
	}
	elt.Reference = strings.Replace(elt.Reference, "\r\n", "\n", -1)
	if !strings.HasSuffix(elt.Reference, "\n") {
		elt.Reference = elt.Reference + "\n"
	}
	if elt.Candidate == "" {
		return fmt.Errorf("Candidate field is required")
	}
	elt.Candidate = strings.Replace(elt.Candidate, "\r\n", "\n", -1)
	if !strings.HasSuffix(elt.Candidate, "\n") {
		elt.Candidate = elt.Candidate + "\n"
	}

	if len(elt.Tests) == 0 {
		return fmt.Errorf("Tests list must not be empty")
	}
	for i, test := range elt.Tests {
		elt.Tests[i] = strings.Replace(test, "\r\n", "\n", -1)
	}

	return nil
}

func (req *Request) RunTest(input, source string) (*TestResult, error) {
	// create a sandbox directory
	dirname, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("Failed to create working directory: %v", err)
	}
	defer os.RemoveAll(dirname)

	// set up the environment files
	stdin := strings.NewReader(input)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := ioutil.WriteFile(filepath.Join(dirname, "source.py"), []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("Failed to create source.py file: %v", err)
	}

	// execute the test
	cmd := exec.Command(sandboxpath,
		"-m", strconv.Itoa(req.MaxMB),
		"-c", strconv.Itoa(req.MaxCPUSeconds),
		"--",
		python27path, "source.py",
	)
	cmd.Dir = dirname
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()

	if err == nil {
		// the race is on--watch for the timeout and the process completing on its own
		timer := time.After(time.Duration(req.MaxTotalSeconds) * time.Second)
		terminate := make(chan bool)
		go func() {
			cmd.Wait()
			terminate <- true
		}()

	waitloop:
		for {
			select {
			case <-timer:
				cmd.Process.Kill()
			case <-terminate:
				break waitloop
			}
		}
	}

	result := &TestResult{
		Error:   err != nil || !cmd.ProcessState.Success(),
		Message: cmd.ProcessState.String(),
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}

	return result, nil
}

func grade_python27_inputoutput(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	in := new(Request)
	if err := decoder.Decode(in); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding input: %v", err), http.StatusBadRequest)
		return
	}
	if err := in.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Error validating input: %v", err), http.StatusBadRequest)
		return
	}

	results := []*TestResultPair{}

	for n, test := range in.Tests {
		// run it with the reference solution
		ref, err := in.RunTest(test, in.Reference)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error running reference solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// run it with the candidate solution
		cand, err := in.RunTest(test, in.Candidate)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error running candidate solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		elt := &TestResultPair{
			Success:   !ref.Error && !cand.Error && ref.Stdout == cand.Stdout,
			Reference: ref,
			Candidate: cand,
		}
		results = append(results, elt)
	}

	writeJson(w, r, results)
}

func grade_python27_expression(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	in := new(Request)
	if err := decoder.Decode(in); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding input: %v", err), http.StatusBadRequest)
		return
	}
	if err := in.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Error validating input: %v", err), http.StatusBadRequest)
		return
	}
	for i, elt := range in.Tests {
		if elt == "" {
			http.Error(w, fmt.Sprintf("Error validating test %d: empty expression", i), http.StatusBadRequest)
			return
		}
	}

	results := []*TestResultPair{}

	for n, test := range in.Tests {
		// run it with the reference solution
		ref, err := in.RunTest("", fmt.Sprintf("%sprint repr(%s)", in.Reference, test))
		if err != nil {
			http.Error(w, fmt.Sprintf("Error running reference solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// run it with the candidate solution
		cand, err := in.RunTest("", fmt.Sprintf("%sprint repr(%s)", in.Candidate, test))
		if err != nil {
			http.Error(w, fmt.Sprintf("Error running candidate solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		elt := &TestResultPair{
			Success:   !ref.Error && !cand.Error && ref.Stdout == cand.Stdout,
			Reference: ref,
			Candidate: cand,
		}
		results = append(results, elt)
	}

	writeJson(w, r, results)
}
