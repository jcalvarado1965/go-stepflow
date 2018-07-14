package stepflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/knetic/govaluate"
	"github.com/oliveagle/jsonpath"
)

// ConditionalStep only forwards to the next step if condition is satisfied.
// Input must be JSON.
type ConditionalStep struct {
	BaseStep
	Condition string `json:"condition,omitempty"`
}

// PrepareMarshal sets the step type
func (s *ConditionalStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeConditional
}

// Validate checks the condition is set and can be compiled
func (s *ConditionalStep) Validate() []error {
	_, _, errList := s.getExprAndParams()

	return errList
}

// Do implements DoerStep interface
func (s *ConditionalStep) Do(ctx context.Context, exec Executor, flow *Flow) error {
	var jsonData interface{}
	var err error
	if jsonData, err = getJSONData(flow.Data); err != nil {
		return err
	}

	expr, params, errList := s.getExprAndParams()
	if len(errList) > 0 {
		// should have been caught during validation
		return errors.New("One or more errors getting expression and parameters")
	}

	paramMap := make(map[string]interface{})

	for _, param := range params {
		paramVal, err := jsonpath.JsonPathLookup(jsonData, param)
		if err != nil {
			return err
		}
		paramMap[param] = paramVal
	}

	exprVal, err := expr.Evaluate(paramMap)
	if err != nil {
		return err
	}

	// decide whether to interrupt: if value is false, or nil, or zero, or empty string
	interrupt := false
	switch val := exprVal.(type) {
	case string:
		if val == "" {
			interrupt = true
		}
	case int:
		if val == 0 {
			interrupt = true
		}
	case bool:
		if !val {
			interrupt = true
		}
	case interface{}:
		if val == nil {
			interrupt = true
		}
	}

	if interrupt {
		exec.GetLogger().Infof(ctx, "Expression '%s' evaluated to falsey. Interrupting flow.", s.Condition)
		flow.State = FlowStateInterrupted
	} else {
		exec.GetLogger().Infof(ctx, "Expression '%s' evaluated to truthey. Flow continues.", s.Condition)
	}

	return nil
}

func getJSONData(flowData interface{}) (jsonData interface{}, err error) {
	switch data := flowData.(type) {
	case []byte:
		err = json.Unmarshal(data, &jsonData)
	case string:
		err = json.Unmarshal([]byte(data), &jsonData)
	case json.RawMessage:
		err = json.Unmarshal(data, &jsonData)
	default:
		err = errors.New("Unrecognized data type for conditional")
	}

	return jsonData, err
}

func (s *ConditionalStep) getExprAndParams() (expr *govaluate.EvaluableExpression, params []string, errList []error) {
	if s.Condition == "" {
		return nil, nil, []error{errors.New("Condition is empty")}
	}

	expr, err := govaluate.NewEvaluableExpression(s.Condition)
	if err != nil {
		return nil, nil, []error{fmt.Errorf("Error parsing expression '%s': %s", s.Condition, err.Error())}
	}

	for _, token := range expr.Tokens() {
		if token.Kind == govaluate.VARIABLE {
			_, err = jsonpath.Compile(token.Value.(string))
			if err != nil {
				errList = append(errList, fmt.Errorf("Selector '%s' has compilation errors: %s", token.Value, err.Error()))
			} else {
				params = append(params, token.Value.(string))
			}
		}
	}

	return expr, params, errList
}
