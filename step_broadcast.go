package stepflow

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// BroadcastStep describes a step that takes its input and forwards to
// multiple steps
type BroadcastStep struct {
	BaseStep
	ForwardTo    []Step   `json:"-"`
	ForwardToIDs []string `json:"forwardTo,omitempty"`
}

// PrepareMarshal sets the step type
func (s *BroadcastStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeBroadcast
}

// ResolveIDs resolve the ForwardToIDs
func (s *BroadcastStep) ResolveIDs(stepMap map[string]Step) error {
	s.ForwardTo = []Step{}
	for _, id := range s.ForwardToIDs {
		if step, ok := stepMap[id]; ok {
			s.ForwardTo = append(s.ForwardTo, step)
		} else {
			return fmt.Errorf("StepID %s not found in workflow", id)
		}
	}
	return nil
}

// Split implements the splitter step interface
func (s *BroadcastStep) Split(ctx context.Context, exec Executor, flow *Flow) (outflows []*Flow, split *FlowSplit, err error) {
	split = &FlowSplit{
		ID:            FlowSplitID(uuid.New().String()),
		WorkflowRunID: flow.WorkflowRunID,
		SplitStepID:   s.ID,
		ParentFlowID:  flow.ID,
		IndexType:     FlowSplitKeyIndex,
	}
	newSplits := make([]FlowSplitID, len(flow.Splits))
	copy(newSplits, flow.Splits)
	newSplits = append(newSplits, split.ID)
	for i, f := range s.ForwardTo {
		outflow := &Flow{
			ID:            FlowID(uuid.New().String()),
			WorkflowRunID: flow.WorkflowRunID,
			State:         FlowStateActive,
			Data:          flow.Data,
			ContentType:   flow.ContentType,
			Splits:        newSplits,
			SplitKey:      s.ForwardToIDs[i],
			NextStepID:    f.GetID(),
		}
		split.FlowIDs = append(split.FlowIDs, outflow.ID)
		outflows = append(outflows, outflow)
	}

	return outflows, split, nil
}
