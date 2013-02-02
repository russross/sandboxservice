package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var Python27StdinDescription = &ProblemType{
	Name: "Python 2.7 Stdin",
	Tag:  "python27stdin",
	FieldList: []ProblemField{
		{
			Name:    "Description",
			Prompt:  "Enter the problem description here",
			Title:   "Problem description",
			Type:    "markdown",
			Creator: "edit",
			Student: "view",
			Grader:  "nothing",
			Result:  "view",
		},
		{
			Name:    "Reference",
			Prompt:  "Enter the reference solution here",
			Title:   "Reference solution",
			Type:    "python",
			Creator: "edit",
			Student: "nothing",
			Grader:  "view",
			Result:  "nothing",
		},
		{
			Name:    "Candidate",
			Prompt:  "Enter your solution here",
			Title:   "Student solution",
			Type:    "python",
			Creator: "nothing",
			Student: "edit",
			Grader:  "view",
			Result:  "view",
		},
		{
			Name:    "Tests",
			Prompt:  "Test cases",
			Title:   "This data will be given to you via Stdin",
			Type:    "text",
			List:    true,
			Creator: "edit",
			Student: "view",
			Grader:  "view",
			Result:  "view",
		},
		{
			Name:    "MaxSeconds",
			Prompt:  "Max time permitted in seconds",
			Title:   "Max time permitted in seconds",
			Type:    "int",
			Default: "10",
			Creator: "edit",
			Student: "view",
			Grader:  "view",
			Result:  "view",
		},
		{
			Name:    "MaxMB",
			Prompt:  "Max memory permitted in megabytes",
			Title:   "Max memory permitted in megabytes",
			Type:    "int",
			Default: "32",
			Creator: "edit",
			Student: "view",
			Grader:  "view",
			Result:  "view",
		},
		{
			Name:    "Report",
			Prompt:  "Grader report",
			Title:   "Grader report",
			Type:    "text",
			Creator: "nothing",
			Student: "nothing",
			Grader:  "edit",
			Result:  "view",
		},
		{
			Name:    "Passed",
			Prompt:  "Did the solution pass?",
			Title:   "Did the solution pass?",
			Type:    "bool",
			Creator: "nothing",
			Student: "nothing",
			Grader:  "edit",
			Result:  "view",
		},
	},
}

type Python27StdinRequest struct {
	Reference  string
	Candidate  string
	Tests      []string
	MaxSeconds int
	MaxMB      int
}

type Python27StdinResponse struct {
	Report string
	Passed bool
}

type TestResult struct {
	Error   bool
	Message string
	Stdout  string
	Stderr  string
}

type Request struct {
	MaxSeconds int
	MaxMB      int

	Reference string
	Candidate string

	Tests []string
}

func (elt *Python27StdinRequest) Validate() error {
	// check Reference solution
	if elt.Reference == "" {
		return fmt.Errorf("Reference solution is required")
	}
	elt.Reference = strings.Replace(elt.Reference, "\r\n", "\n", -1)
	if !strings.HasSuffix(elt.Reference, "\n") {
		elt.Reference = elt.Reference + "\n"
	}

	// check Candidate solution
	if elt.Candidate == "" {
		return fmt.Errorf("Candidate solution is required")
	}
	elt.Candidate = strings.Replace(elt.Candidate, "\r\n", "\n", -1)
	if !strings.HasSuffix(elt.Candidate, "\n") {
		elt.Candidate = elt.Candidate + "\n"
	}

	// check Test list
	if len(elt.Tests) == 0 {
		return fmt.Errorf("Tests list must not be empty")
	}
	for i, test := range elt.Tests {
		test = strings.Replace(test, "\r\n", "\n", -1)
		if !strings.HasSuffix(test, "\n") {
			test = test + "\n"
		}
		elt.Tests[i] = test
	}

	// check MaxSeconds
	if elt.MaxSeconds < 1 {
		return fmt.Errorf("MaxSeconds must be >= 1")
	} else if elt.MaxSeconds > MaxSeconds {
		return fmt.Errorf("MaxSeconds must be <= %d", MaxSeconds)
	}

	// check MaxMB
	if elt.MaxMB < 1 {
		return fmt.Errorf("MaxMB must be >= 1")
	} else if elt.MaxMB > MaxMB {
		return fmt.Errorf("MaxMB must be <= %d", MaxMB)
	}

	return nil
}

func (req *Python27StdinRequest) RunTest(input, source string) (*TestResult, error) {
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
	if err := ioutil.WriteFile(filepath.Join(dirname, "student.py"), []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("Failed to create student.py file: %v", err)
	}

	// execute the test
	cmd := exec.Command(SandboxPath,
		"-m", strconv.Itoa(req.MaxMB),
		"-c", strconv.Itoa(req.MaxSeconds+1),
		"--",
		Python27Path, "student.py",
	)
	cmd.Dir = dirname
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	killed := false

	if err == nil {
		// the race is on--watch for the timeout and the process completing on its own
		timer := time.After(time.Duration(req.MaxSeconds) * time.Second)
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
				killed = true
			case <-terminate:
				break waitloop
			}
		}
	}

	message := ""
	if err != nil {
		message = err.Error()
	} else if killed {
		message = "Process exceeded its time limit"
	} else if !cmd.ProcessState.Success() {
		message = cmd.ProcessState.String()
	}

	result := &TestResult{
		Error:   err != nil || !cmd.ProcessState.Success(),
		Message: message,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}

	return result, nil
}

func python27stdin_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	request := new(Python27StdinRequest)
	if err := decoder.Decode(request); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding input: %v", err), http.StatusBadRequest)
		return
	}
	if err := request.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Error validating input: %v", err), http.StatusBadRequest)
		return
	}

	response := &Python27StdinResponse{
		Report: "",
		Passed: true,
	}

	passcount := 0
	for n, test := range request.Tests {
		// run it with the reference solution
		ref, err := request.RunTest(test, request.Reference)
		if err != nil {
			log.Printf("Error running reference solution %d: %v", n, err)
			http.Error(w, fmt.Sprintf("Error running reference solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// run it with the candidate solution
		cand, err := request.RunTest(test, request.Candidate)
		if err != nil {
			log.Printf("Error running candidate solution %d: %v", n, err)
			http.Error(w, fmt.Sprintf("Error running candidate solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// report the result
		if n > 0 {
			response.Report += "\n-=-=-=-=-=-=-=-=-\n\n"
		}

		// record a pass or fail
		if ref.Error || cand.Error || ref.Stdout != cand.Stdout {
			response.Report += fmt.Sprintf("Test #%d: FAILED\n", n+1)
			response.Passed = false
		} else {
			response.Report += fmt.Sprintf("Test #%d: PASSED\n", n+1)
			passcount++
		}

		// give a few details
		if ref.Error {
			response.Report += fmt.Sprintf("The reference solution ended in error: %s\n", ref.Message)
		}
		if cand.Error {
			response.Report += fmt.Sprintf("The candidate solution ended in error: %s\n", cand.Message)
		}
		if !ref.Error && !cand.Error && ref.Stdout != cand.Stdout {
			response.Report += fmt.Sprintf("The output was incorrect.\n\n"+
				"The correct output is:\n[[[[\n%s]]]]\n\n"+
				"Your output was:\n[[[[\n%s]]]]\n",
				ref.Stdout,
				cand.Stdout)
		}
	}
	if len(request.Tests) == 1 {
		log.Printf("  passed %d/%d test", passcount, len(request.Tests))
	} else {
		log.Printf("  passed %d/%d tests", passcount, len(request.Tests))
	}

	writeJson(w, r, response)
}
