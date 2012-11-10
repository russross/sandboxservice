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
	"strings"
	"time"
)

const python27name = "bin/python2.7-static"

var python27path string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic("Failed to find working directory: " + err.Error())
	}
	python27path = filepath.Join(wd, python27name)
}

type InputOutputTest string

func (elt InputOutputTest) Validate() error {
	return nil
}

type TestResult struct {
	Killed     bool
	ExitStatus int
	Message    string
	Stdout     string
	Return     string
	Stderr     string
}

type InputOutputTestResult struct {
	Success bool

	CPUSeconds   float64
	TotalSeconds float64
	MB           float64

	Reference *TestResult
	Candidate *TestResult
}

type Request struct {
	MaxCPUSeconds   float64
	MaxTotalSeconds float64
	MaxMB           float64

	Reference string
	Candidate string

	Tests []InputOutputTest
}

func (elt *Request) Validate() error {
	if elt.MaxCPUSeconds < 0.1 {
		return fmt.Errorf("MaxCPUSeconds must be >= 0.1")
	} else if elt.MaxCPUSeconds > 60.0 {
		return fmt.Errorf("MaxCPUSeconds must be <= 60.0")
	}

	if elt.MaxTotalSeconds < 0.1 {
		return fmt.Errorf("MaxTotalSeconds must be >= 0.1")
	} else if elt.MaxTotalSeconds > 60.0 {
		return fmt.Errorf("MaxTotalSeconds must be <= 60.0")
	}

	if elt.MaxMB < 1.0 {
		return fmt.Errorf("MaxMB must be >= 1.0")
	} else if elt.MaxMB > 1024.0 {
		return fmt.Errorf("MaxMB must be <= 1024.0")
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
	if err := ioutil.WriteFile(filepath.Join(dirname, "source.py"), []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("Failed to create source.py file: %v", err)
	}

	// execute the test
	start := time.Now()
	cmd := exec.Command(python27path, "source.py")
	cmd.Dir = dirname
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	finish := time.Now()
	elapsed := finish.Sub(start).Seconds()

	success := err == nil && cmd.ProcessState.Success()
	exit := 0
	if !success {
		exit = -1
	}
	testres := &TestResult{
		Killed:     !success,
		ExitStatus: exit,
		Message:    cmd.ProcessState.String(),
		Stdout:     stdout.String(),
		Return:     "",
		Stderr:     stderr.String(),
	}

	result := &InputOutputTestResult{
		Success: success,

		CPUSeconds:   (cmd.ProcessState.SystemTime() + cmd.ProcessState.UserTime()).Seconds(),
		TotalSeconds: elapsed,
		MB:           0.0,

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
