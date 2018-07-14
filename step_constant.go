package stepflow

import (
	"context"
	"encoding/json"
	"fmt"
)

// ConstantStep takes its value (JSON) and sends it to the Next step
type ConstantStep struct {
	BaseStep
	Value json.RawMessage `json:"value,omitempty"`
}

// PrepareMarshal sets the step type
func (s *ConstantStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeConstant
}

// Validate checks that the constant step has a value
func (s *ConstantStep) Validate() []error {
	if s.Value == nil {
		return []error{fmt.Errorf("Missing constant value in step ID %s", s.ID)}
	}

	return []error{}
}

// Do implements DoerStep interface
func (s *ConstantStep) Do(ctx context.Context, exec Executor, flow *Flow) error {
	flow.Data = s.Value
	flow.ContentType = "application/json"
	exec.GetLogger().Debugf(ctx, "Constant step ID %s returned value %s", s.GetID(), string(s.Value))

	return nil
}
