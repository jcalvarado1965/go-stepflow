package stepflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// DistributeStep takes its input, which must be a JSON array, and
// sends each element of the array to the Next step
type DistributeStep struct {
	BaseStep
}

type rawMessageArray []json.RawMessage
type rawMessageMap map[string]json.RawMessage

// PrepareMarshal sets the step type
func (s *DistributeStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeDistribute
}

// Split implements the splitter step interface
func (s *DistributeStep) Split(ctx context.Context, exec Executor, flow *Flow) (outflows []*Flow, split *FlowSplit, err error) {
	// make sure flow data can be split: either dictionary or array
	var dataBytes []byte
	switch d := flow.Data.(type) {
	case []byte:
		dataBytes = d
		break
	case json.RawMessage:
		dataBytes = []byte(d)
		break
	case string:
		dataBytes = []byte(d)
		break
	default:
		return nil, nil, errors.New("Unsupported data type for distribute")
	}
	decoder := json.NewDecoder(bytes.NewReader(dataBytes))
	token, err := decoder.Token()
	if err != nil {
		return nil, nil, err
	}

	delim, ok := token.(json.Delim)
	if !ok {
		return nil, nil, errors.New("First token in flow data is not a delimeter")
	}

	split = &FlowSplit{
		ID:            FlowSplitID(uuid.New().String()),
		WorkflowRunID: flow.WorkflowRunID,
		SplitStepID:   s.ID,
		ParentFlowID:  flow.ID,
	}

	newSplits := make([]FlowSplitID, len(flow.Splits))
	copy(newSplits, flow.Splits)
	newSplits = append(newSplits, split.ID)
	if delim.String() == "[" {
		var messages rawMessageArray
		split.IndexType = FlowSplitNumericalIndex
		if err = json.Unmarshal(dataBytes, &messages); err != nil {
			return nil, nil, err
		}
		for i, v := range messages {
			outflow := &Flow{
				ID:            FlowID(uuid.New().String()),
				WorkflowRunID: flow.WorkflowRunID,
				State:         FlowStateActive,
				Data:          v,
				ContentType:   "application/json",
				Splits:        newSplits,
				SplitIndex:    i,
				NextStepID:    s.NextID,
			}
			split.FlowIDs = append(split.FlowIDs, outflow.ID)
			outflows = append(outflows, outflow)
		}
	} else { // must be {
		var messageMap rawMessageMap
		split.IndexType = FlowSplitKeyIndex
		if err = json.Unmarshal(dataBytes, &messageMap); err != nil {
			return nil, nil, err
		}
		for k, v := range messageMap {
			outflow := &Flow{
				ID:            FlowID(uuid.New().String()),
				WorkflowRunID: flow.WorkflowRunID,
				State:         FlowStateActive,
				Data:          v,
				ContentType:   "application/json",
				Splits:        newSplits,
				SplitKey:      k,
				NextStepID:    s.NextID,
			}
			split.FlowIDs = append(split.FlowIDs, outflow.ID)
			outflows = append(outflows, outflow)
		}
	}

	return outflows, split, nil
}
