package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

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

	Tests []*InputOutputTest
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

	result := &InputOutputTestResult{
		Success: true,

		CPUSeconds:   0.73,
		TotalSeconds: 0.74,
		MB:           8.125,

		Reference: &TestResult{
			Killed:     false,
			ExitStatus: 0,
			Message:    "",
			Stdout:     "Hello, world\n",
			Return:     "",
			Stderr:     "",
		},

		Candidate: &TestResult{
			Killed:     false,
			ExitStatus: 0,
			Message:    "",
			Stdout:     "Hello, world\n",
			Return:     "",
			Stderr:     "",
		},
	}
	writeJson(w, r, result)
}
