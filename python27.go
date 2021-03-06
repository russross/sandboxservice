package main

import (
	"bytes"
	"crypto/sha1"
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

// maps hash of testtype:referencesolution:testdata to *TestResult
// only used for reference solutions
var cache = make(map[string]*TestResult)

var Python27ModuleDescription = &ProblemType{
	Name: "Python 2.7 Module",
	Tag:  "python27module",
	FieldList: []ProblemField{
		{
			Name:    "Passed",
			Prompt:  "Did the solution pass?",
			Title:   "Did the solution pass?",
			Type:    "bool",
			Creator: "nothing",
			Student: "view",
			Grader:  "edit",
			Result:  "view",
		},
		{
			Name:    "Report",
			Prompt:  "Grader report",
			Title:   "Grader report",
			Type:    "text",
			Creator: "nothing",
			Student: "view",
			Grader:  "edit",
			Result:  "view",
		},
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
			Prompt:  "Test drivers",
			Title:   "This code will run and will access your code as the 'Candidate' module",
			Type:    "python",
			List:    true,
			Creator: "edit",
			Student: "view",
			Grader:  "view",
			Result:  "view",
		},
		{
			Name:    "Output",
			Prompt:  "Expected output",
			Title:   "This is the output produced by the reference solution",
			Type:    "text",
			List:    true,
			Creator: "nothing",
			Student: "view",
			Grader:  "nothing",
			Result:  "view",
		},
		{
			Name:    "HiddenTests",
			Prompt:  "Hidden test drivers",
			Title:   "This code will also run and access your code as the 'Candidate' module",
			Type:    "python",
			List:    true,
			Creator: "edit",
			Student: "nothing",
			Grader:  "view",
			Result:  "nothing",
		},
		{
			Name:    "MaxSeconds",
			Prompt:  "Max time permitted in seconds",
			Title:   "Max time permitted in seconds",
			Type:    "int",
			Default: "2",
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
	},
}

var Python27StdinDescription = &ProblemType{
	Name: "Python 2.7 Stdin",
	Tag:  "python27stdin",
	FieldList: []ProblemField{
		{
			Name:    "Passed",
			Prompt:  "Did the solution pass?",
			Title:   "Did the solution pass?",
			Type:    "bool",
			Creator: "nothing",
			Student: "view",
			Grader:  "edit",
			Result:  "view",
		},
		{
			Name:    "Report",
			Prompt:  "Grader report",
			Title:   "Grader report",
			Type:    "text",
			Creator: "nothing",
			Student: "view",
			Grader:  "edit",
			Result:  "view",
		},
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
			Name:    "Output",
			Prompt:  "Expected output",
			Title:   "This is the output produced by the reference solution",
			Type:    "text",
			List:    true,
			Creator: "nothing",
			Student: "view",
			Grader:  "nothing",
			Result:  "view",
		},
		{
			Name:    "HiddenTests",
			Prompt:  "Hidden test cases",
			Title:   "This data will also be given to you via Stdin",
			Type:    "text",
			List:    true,
			Creator: "edit",
			Student: "nothing",
			Grader:  "view",
			Result:  "nothing",
		},
		{
			Name:    "MaxSeconds",
			Prompt:  "Max time permitted in seconds",
			Title:   "Max time permitted in seconds",
			Type:    "int",
			Default: "2",
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
	},
}

type Python27CommonRequest struct {
	Reference   string
	Candidate   string
	Tests       []string
	HiddenTests []string
	MaxSeconds  int
	MaxMB       int
}

func (elt *Python27CommonRequest) Validate() error {
	// check Reference solution
	elt.Reference = fixLineEndings(elt.Reference)
	if isEmpty(elt.Reference) {
		return fmt.Errorf("Reference solution is required")
	}

	// check Candidate solution
	elt.Candidate = fixLineEndings(elt.Candidate)

	// check Test list
	lst := []string{}
	for _, test := range elt.Tests {
		test = fixLineEndings(test)
		if !isEmpty(test) {
			lst = append(lst, test)
		}
	}
	elt.Tests = lst
	if len(elt.Tests) == 0 {
		return fmt.Errorf("Tests list must not be empty")
	}

	// check HiddenTest list
	lst = []string{}
	for _, test := range elt.HiddenTests {
		test = fixLineEndings(test)
		if !isEmpty(test) {
			lst = append(lst, test)
		}
	}
	elt.HiddenTests = lst

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

func (req *Python27CommonRequest) RunReferenceTest(test, source string, isModule bool) (*TestResult, error) {
	// create a signature
	h := sha1.New()
	if isModule {
		fmt.Fprintf(h, "python27module")
	} else {
		fmt.Fprintf(h, "python27stdin")
	}
	fmt.Fprintf(h, "\ue000%s\ue000%s", source, test)
	key := fmt.Sprintf("%x", h.Sum(nil))
	if result, present := cache[key]; present {
		return result, nil
	}
	result, err := req.RunTest(test, source, isModule)
	if err == nil {
		cache[key] = result
	}
	return result, err
}

func (req *Python27CommonRequest) RunTest(test, source string, isModule bool) (*TestResult, error) {
	// create a sandbox directory
	dirname, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("Failed to create working directory: %v", err)
	}
	defer os.RemoveAll(dirname)

	// set up the environment files
	stdinData := ""
	if !isModule {
		stdinData = test
	}
	stdin := strings.NewReader(stdinData)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if isModule {
		if err := ioutil.WriteFile(filepath.Join(dirname, "main.py"), []byte(test), 0644); err != nil {
			return nil, fmt.Errorf("Failed to create main.py file: %v", err)
		}
		if err := ioutil.WriteFile(filepath.Join(dirname, "Candidate.py"), []byte(source), 0644); err != nil {
			return nil, fmt.Errorf("Failed to create Candidate.py file: %v", err)
		}
	} else {
		if err := ioutil.WriteFile(filepath.Join(dirname, "main.py"), []byte(source), 0644); err != nil {
			return nil, fmt.Errorf("Failed to create main.py file: %v", err)
		}
	}

	// execute the test
	cmd := exec.Command(SandboxPath,
		"-m", strconv.Itoa(req.MaxMB),
		"-c", strconv.Itoa(req.MaxSeconds+1),
		"--",
		Python27Path, "main.py",
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

func python27module_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	python27_common_handler(w, r, decoder, true)
}

func python27stdin_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	python27_common_handler(w, r, decoder, false)
}

func python27_common_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder, isModule bool) {
	request := new(Python27CommonRequest)
	if err := decoder.Decode(request); err != nil {
		log.Printf("Error decoding input: %v", err)
		http.Error(w, fmt.Sprintf("Error decoding input: %v", err), http.StatusBadRequest)
		return
	}
	if err := request.Validate(); err != nil {
		log.Printf("Error validating input: %v", err)
		http.Error(w, fmt.Sprintf("Error validating input: %v", err), http.StatusBadRequest)
		return
	}

	response := &GenericResponse{
		Report: "",
		Passed: true,
	}

	passcount := 0
	for n, test := range request.Tests {
		// run it with the reference solution
		ref, err := request.RunReferenceTest(test, request.Reference, isModule)
		if err != nil {
			log.Printf("Error running reference solution %d: %v", n, err)
			http.Error(w, fmt.Sprintf("Error running reference solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// run it with the candidate solution
		cand, err := request.RunTest(test, request.Candidate, isModule)
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
			if ref.Stdout != "" {
				response.Report += fmt.Sprintf("Standard output before it quit:\n<<<<\n%s>>>>\n\n", ref.Stdout)
			}
			if ref.Stderr != "" {
				response.Report += fmt.Sprintf("Standard error reported:\n<<<<\n%s>>>>\n\n", ref.Stderr)
			}
		}
		if cand.Error {
			response.Report += fmt.Sprintf("The candidate solution ended in error: %s\n", cand.Message)
			if cand.Stdout != "" {
				response.Report += fmt.Sprintf("Standard output before it quit:\n<<<<\n%s>>>>\n\n", cand.Stdout)
			}
			if cand.Stderr != "" {
				response.Report += fmt.Sprintf("Standard error reported:\n<<<<\n%s>>>>\n\n", cand.Stderr)
			}
		}
		if !ref.Error && !cand.Error && ref.Stdout != cand.Stdout {
			response.Report += fmt.Sprintf("The output was incorrect.\n\n"+
				"The correct output is:\n<<<<\n%s>>>>\n\n"+
				"Your output was:\n<<<<\n%s>>>>\n",
				ref.Stdout,
				cand.Stdout)
		}
	}
	for n, test := range request.HiddenTests {
		// run it with the reference solution
		ref, err := request.RunReferenceTest(test, request.Reference, isModule)
		if err != nil {
			log.Printf("Error running reference solution on hidden %d: %v", n, err)
			http.Error(w, fmt.Sprintf("Error running reference solution on hidden %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// run it with the candidate solution
		cand, err := request.RunTest(test, request.Candidate, isModule)
		if err != nil {
			log.Printf("Error running candidate solution on hidden %d: %v", n, err)
			http.Error(w, fmt.Sprintf("Error running candidate solution on hidden %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// report the result
		response.Report += "\n-=-=-=-=-=-=-=-=-\n\n"

		// record a pass or fail
		if ref.Error || cand.Error || ref.Stdout != cand.Stdout {
			response.Report += fmt.Sprintf("Hidden test #%d: FAILED\n", n+1)
			response.Passed = false
		} else {
			response.Report += fmt.Sprintf("Hidden test #%d: PASSED\n", n+1)
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
			response.Report += "The output was incorrect.\n"
		}
	}
	tests := len(request.Tests) + len(request.HiddenTests)
	if tests == 1 {
		log.Printf("  passed %d/%d test", passcount, tests)
	} else {
		log.Printf("  passed %d/%d tests", passcount, tests)
	}

	writeJson(w, r, response)
}

func python27module_output_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	python27_common_output_handler(w, r, decoder, true)
}

func python27stdin_output_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder) {
	python27_common_output_handler(w, r, decoder, false)
}

func python27_common_output_handler(w http.ResponseWriter, r *http.Request, decoder *json.Decoder, isModule bool) {
	request := new(Python27CommonRequest)
	if err := decoder.Decode(request); err != nil {
		log.Printf("Error decoding input: %v", err)
		http.Error(w, fmt.Sprintf("Error decoding input: %v", err), http.StatusBadRequest)
		return
	}
	if err := request.Validate(); err != nil {
		log.Printf("Error validating input: %v", err)
		http.Error(w, fmt.Sprintf("Error validating input: %v", err), http.StatusBadRequest)
		return
	}

	results := []string{}

	for n, test := range request.Tests {
		// run it with the reference solution
		ref, err := request.RunReferenceTest(test, request.Reference, isModule)
		if err != nil {
			log.Printf("Error running reference solution %d: %v", n, err)
			http.Error(w, fmt.Sprintf("Error running reference solution %d: %v", n, err), http.StatusInternalServerError)
			return
		}

		// give a few details
		if ref.Error {
			msg := fmt.Sprintf("The reference solution ended in error: %s\n", ref.Message)
			if ref.Stdout != "" {
				msg += fmt.Sprintf("Standard output before it quit:\n<<<<\n%s>>>>\n\n", ref.Stdout)
			}
			if ref.Stderr != "" {
				msg += fmt.Sprintf("Standard error reported:\n<<<<\n%s>>>>\n\n", ref.Stderr)
			}
			results = append(results, msg)
		} else {
			results = append(results, ref.Stdout)
		}
	}

	response := map[string][]string{"Output": results}

	writeJson(w, r, response)
}
