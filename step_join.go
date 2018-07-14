package stepflow

import (
	"context"
	"errors"
	"fmt"
)

// JoinStep waits until all the steps providing input to it complete,
// combines the inputs into a single result and forwards to the Next
// step. The combination is a JSON object whose keys are the IDs of
// the steps providing the input, and the values are the inputs. If
// the input value is not JSON then it is given as a base64-encoded
// string. If the input is from a step receiving a distribution, the
// value is an array.
type JoinStep struct {
	BaseStep
}

// PrepareMarshal sets the step type
func (s *JoinStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeJoin
}

// Join implements Joiner interface for join step
func (s *JoinStep) Join(ctx context.Context, exec Executor, flow *Flow) (joinedFlow *Flow, err error) {
	split, err := flow.getLastSplit(ctx, exec)
	if err != nil {
		return nil, err
	}

	var errIncr int64
	if flow.State == FlowStateError {
		errIncr = 1
	}
	totalFinish, totalError := exec.GetStorage().IncrementWithError(ctx, string(split.ID), 1, errIncr)

	if totalFinish < int64(len(split.FlowIDs)) {
		// not all flows finished
		return nil, nil
	}

	// all flows are finished. figure out join state, and compile results
	flows, _, err := flow.getSiblingFlows(ctx, exec)

	joinedFlow, ok := (exec.GetStorage().RetrieveFlows(ctx, []FlowID{split.ParentFlowID}))[split.ParentFlowID]
	if !ok {
		return nil, fmt.Errorf("Could not retrieve parent flow with ID %s", split.ParentFlowID)
	}

	// if we have all the flows and no errors, treat as successful join
	// and collect all the data
	if len(flows) == len(split.FlowIDs) && totalError == 0 {
		if split.IndexType == FlowSplitKeyIndex {
			dataMap := make(map[string]interface{})
			joinedFlow.Data = dataMap
			for _, flow := range flows {
				if flow.State != FlowStateInterrupted {
					dataMap[flow.SplitKey] = flow.Data
				}
			}
		} else { // it is numerical index
			var dataArr []interface{}
			for _, flow := range flows {
				if flow.State != FlowStateInterrupted {
					dataArr = append(dataArr, flow.Data)
				}
			}
			joinedFlow.Data = dataArr
		}
	} else {
		// some flows in error, or an unusual condition (could not retrieve all flows)
		if totalError > 0 {
			return joinedFlow, errors.New("One or more joined flows finished with errors")
		}
		return joinedFlow, errors.New("Retrieved flows did not match flow count")
	}
	return joinedFlow, nil
}
