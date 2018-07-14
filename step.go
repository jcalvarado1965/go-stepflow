package stepflow

import (
	"context"
	"encoding/json"
	"fmt"
)

// StepType is an enum for the known step types
type StepType string

// These are the known step types
const (
	TypeWebMethod   StepType = "web-method"
	TypeDistribute  StepType = "distribute"
	TypeBroadcast   StepType = "broadcast"
	TypeSelect      StepType = "select"
	TypeConditional StepType = "conditional"
	TypeJoin        StepType = "join"
	TypeRace        StepType = "race"
	TypeConstant    StepType = "constant"
)

// const StepRunKind = "StepRun"

// Step is the interface implemented by all steps
type Step interface {
	GetID() string
	GetNextID() string
	PrepareMarshal()
	ResolveIDs(map[string]Step) error
	Validate() []error
}

// DoerStep is implemented by steps that perform an action
type DoerStep interface {
	Do(ctx context.Context, exec Executor, flow *Flow) error
}

// SplitterStep is implemented by steps that split flows
type SplitterStep interface {
	Split(ctx context.Context, exec Executor, flow *Flow) (outflows []*Flow, split *FlowSplit, err error)
}

// JoinerStep is implemented by steps that join flows
type JoinerStep interface {
	Join(ctx context.Context, exec Executor, flow *Flow) (joinedFlow *Flow, err error)
}

// BaseStep holds the basic step details. ID must be unique within a
// Dataflow. Type is used for serialization. Next points to the
// next step in the workflow (except for BroadcastStep which forwards to
// multiples). AcceptContent indicates the content type the step
// requires, if any. ContentType indicates the content type of
// the output, if any. If KeepOutput is true, the outputs are copied to
// the workflow run so they are available when the workflow completes.
// If HandleErrorAs is not nil, then errors do not stop the flow but
// instead the given JSON is passed to the next task(s)
type BaseStep struct {
	ID          string   `json:"id,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        StepType `json:"type,omitempty"`
	Next        Step     `json:"-"`
	NextID      string   `json:"next,omitempty"`
	// not implemented
	// KeepOutput    bool            `json:"keepOutput,omitempty"`
	// HandleErrorAs json.RawMessage `json:"handleErrorAs,omitempty"`
}

// GetID for Step impl in BaseStep
func (s *BaseStep) GetID() string {
	return s.ID
}

// GetNextID for Step impl in BaseStep
func (s *BaseStep) GetNextID() string {
	return s.NextID
}

// PrepareMarshal for Step impl in BaseStep
func (s *BaseStep) PrepareMarshal() {
	if s.Next != nil {
		s.NextID = s.Next.GetID()
	}
}

// Validate for Step impl in BaseStep
func (s *BaseStep) Validate() []error {
	return []error{}
}

// ResolveIDs for Step impl in BaseStep
func (s *BaseStep) ResolveIDs(stepMap map[string]Step) error {
	if s.NextID != "" {
		var ok bool
		s.Next, ok = stepMap[s.NextID]
		if !ok {
			return fmt.Errorf("StepID %s not found in workflow", s.NextID)
		}
	}
	return nil
}

func (s *BaseStep) String() string {
	var nextID string
	if s.Next != nil {
		nextID = s.Next.GetID()
	}
	return fmt.Sprintf("{ID: %s, Desc: %s, Type: %s, Next: %s}",
		s.ID, s.Description, s.Type, nextID)
}

// UnmarshalStep returns a specialized step based on the raw JSON
func UnmarshalStep(raw json.RawMessage) (Step, error) {
	var base BaseStep
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, err
	}
	switch base.Type {
	case TypeBroadcast:
		var step BroadcastStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeConditional:
		var step ConditionalStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeSelect:
		var step SelectStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeConstant:
		var step ConstantStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeDistribute:
		var step DistributeStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeJoin:
		var step JoinStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeRace:
		var step RaceStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	case TypeWebMethod:
		var step WebMethodStep
		if err := json.Unmarshal(raw, &step); err != nil {
			return nil, err
		}
		return &step, nil
	}

	return nil, fmt.Errorf("Step type not recognized: %s", base.Type)
}
