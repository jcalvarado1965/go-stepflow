package stepflow

import (
	"context"
	"errors"

	"github.com/oliveagle/jsonpath"
)

// SelectStep expects the flow data to be parseable as JSON. It then uses the
// selector expression (see https://github.com/oliveagle/jsonpath) and returns
// the results of applying the expression to the flow data.
type SelectStep struct {
	BaseStep
	Selector string `json:"selector,omitempty"`
}

// PrepareMarshal sets the step type
func (s *SelectStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeSelect
}

// Validate checks the selector can be compiled
func (s *SelectStep) Validate() (errList []error) {
	if s.Selector == "" {
		return []error{errors.New("Selector is empty")}
	}

	if _, err := jsonpath.Compile(s.Selector); err != nil {
		errList = append(errList, err)
	}

	return errList
}

// Do implements DoerStep interface
func (s *SelectStep) Do(ctx context.Context, exec Executor, flow *Flow) error {
	var jsonData interface{}
	var err error
	if jsonData, err = getJSONData(flow.Data); err != nil {
		return err
	}

	flow.Data, err = jsonpath.JsonPathLookup(jsonData, s.Selector)
	return err
}
