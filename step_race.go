package stepflow

import (
	"context"
	"fmt"
)

// RaceStep waits until it receives its first active flow input and forwards it
// to the Next step. Subsequent inputs are discarded
type RaceStep struct {
	BaseStep
}

// PrepareMarshal sets the step type
func (s *RaceStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeRace
}

// Join implements Joiner interface for race step
func (s *RaceStep) Join(ctx context.Context, exec Executor, flow *Flow) (joinedFlow *Flow, err error) {
	split, err := flow.getLastSplit(ctx, exec)
	if err != nil {
		return nil, err
	}

	var activeFlowCount int64
	var incr int64
	if flow.State == FlowStateActive {
		incr = 1
	}
	activeFlowCount, _ = exec.GetStorage().IncrementWithError(ctx, s.ID+":"+string(split.ID), incr, 0)

	totalFinish, _ := exec.GetStorage().IncrementWithError(ctx, string(split.ID), 1, 0)

	// let the first active flow through
	if activeFlowCount == 1 && incr == 1 {
		joinedFlow, ok := (exec.GetStorage().RetrieveFlows(ctx, []FlowID{split.ParentFlowID}))[split.ParentFlowID]
		if !ok {
			return nil, fmt.Errorf("Could not retrieve parent flow with ID %s", split.ParentFlowID)
		}
		exec.GetLogger().Debugf(ctx, "Race won by flow %s", flow.ID)
		joinedFlow.Data = flow.Data
		return joinedFlow, nil
	} else if activeFlowCount == 0 && totalFinish == int64(len(split.FlowIDs)) {
		// we've run out of flows, so interrupt
		joinedFlow, ok := (exec.GetStorage().RetrieveFlows(ctx, []FlowID{split.ParentFlowID}))[split.ParentFlowID]
		if !ok {
			return nil, fmt.Errorf("Could not retrieve parent flow with ID %s", split.ParentFlowID)
		}
		joinedFlow.Data = nil
		joinedFlow.State = FlowStateInterrupted
		exec.GetLogger().Debugf(ctx, "No flow won the race. Interrupting")
		return joinedFlow, nil
	} else {
		// loser flows, so ignore
		return nil, nil
	}
}
