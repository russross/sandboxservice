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

type InputOutputTest string

func (elt InputOutputTest) Validate() error {
	return nil
}

type TestResult struct {
	Error   bool
	Message string
	Stdout  string
	Stderr  string
}

type InputOutputTestResult struct {
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

	Tests []InputOutputTest
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
	if elt.Candidate == "" {
		return fmt.Errorf("Candidate field is required")
	}

	if len(elt.Tests) == 0 {
		return fmt.Errorf("Tests list must not be empty")
	}

	for i, test := range elt.Tests {
		if err := test.Validate(); err != nil {
			return fmt.Errorf("Error validating test %d: %v", i, err)
		}
	}

	return nil
}

func (req *Request) RunTest(test InputOutputTest, ref *TestResult) (*InputOutputTestResult, error) {
	// create a sandbox directory
	dirname, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("Failed to create working directory: %v", err)
	}
	defer os.RemoveAll(dirname)

	// set up the environment files
	stdin := strings.NewReader(string(test))
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	source := req.Candidate
	if ref == nil {
		source = req.Reference
	}
	if err := ioutil.WriteFile(filepath.Join(dirname, "source.py"), []byte(source+"\n"), 0644); err != nil {
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

	failed := err != nil || !cmd.ProcessState.Success()
	testres := &TestResult{
		Error:   failed,
		Message: cmd.ProcessState.String(),
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}

	result := &InputOutputTestResult{
		Success:   !failed,
		Reference: ref,
		Candidate: testres,
	}

	if ref == nil {
		result.Reference = testres
		result.Candidate = nil
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

	results := []*InputOutputTestResult{}

	for n, test := range in.Tests {
		// run it once with the reference solution
		ref, err := in.RunTest(test, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error running reference solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}
		cand, err := in.RunTest(test, ref.Reference)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error running candidate solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}
		cand.Success = ref.Success && cand.Success && cand.Candidate.Stdout == cand.Reference.Stdout
		results = append(results, cand)
	}

	writeJson(w, r, results)
}
