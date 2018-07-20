package stepflow

import (
	"context"
	"errors"
	"fmt"
)

// FlowState represents the state of a flow
type FlowState string

// These are the states a flow can be in, w.r.t. to the previous step ID
const (
	FlowStateActive      FlowState = "Active"
	FlowStateError       FlowState = "Error"       // flow stopped due to error
	FlowStateCompleted   FlowState = "Completed"   // flow dead-ended
	FlowStateSplit       FlowState = "Split"       // there are child flows
	FlowStateInterrupted FlowState = "Interrupted" // e.g. from a conditional
)

// FlowSplitIndexType represents the type of flow split index (key or numerical)
type FlowSplitIndexType string

// These are the index types for a flow split
const (
	FlowSplitNumericalIndex FlowSplitIndexType = "Numerical"
	FlowSplitKeyIndex       FlowSplitIndexType = "Key"
)

// FlowSplitID identifies a flow split instance
type FlowSplitID string

// FlowID identifies a flow
type FlowID string

// FlowSplit holds information about an instance of a split flow
type FlowSplit struct {
	ID            FlowSplitID        // uuid
	DataflowRunID DataflowRunID      // identifies the run instance
	SplitStepID   string             // this would be a broadcast or distribute step
	ParentFlowID  FlowID             // the flow that was split
	IndexType     FlowSplitIndexType // the type of index used in the split
	FlowIDs       []FlowID           // lists the flows generated by the split
}

type FlowNoData struct {
	ID             FlowID        // UUID
	DataflowRunID  DataflowRunID // identifies the run instance
	PreviousStepID string
	NextStepID     string
	State          FlowState
	Message        string        // if State is Error, this has the explanation
	ContentType    string        // content type of data
	Splits         []FlowSplitID // identifies the splits that led to this flow
	SplitKey       string        // if the current split is from dictionary, the key
	SplitIndex     int           // if the current split is from array, the index
}

// Flow represents an execution unit for a workflow
// Dataflow runs start with one flow, set at the starting step.
// When a step completes, the flow transitions to the next step.
// Flows can split in Distribute and Broadcast steps.
// Flows can merge in the Join and Race steps.
type Flow struct {
	FlowNoData             // helps with serialization
	Data       interface{} // if State is Completed, this has the result
}

func (f *Flow) isRoot() bool {
	return len(f.Splits) == 0
}

func (f *Flow) getLastSplit(ctx context.Context, exec Executor) (*FlowSplit, error) {
	if f.isRoot() {
		return nil, errors.New("Not a split flow")
	}

	// siblings are the flows in the most recent split
	splitID := f.Splits[len(f.Splits)-1]
	split, ok := exec.GetStorage().RetrieveFlowSplits(ctx, []FlowSplitID{splitID})[splitID]
	if !ok {
		return nil, fmt.Errorf("Could not retrieve split with ID %s", splitID)
	}
	return split, nil
}

func (f *Flow) getSiblingFlows(ctx context.Context, exec Executor) ([]*Flow, *FlowSplit, error) {
	var split *FlowSplit
	var err error
	if split, err = f.getLastSplit(ctx, exec); err != nil {
		return nil, nil, err
	}

	flowMap := exec.GetStorage().RetrieveFlows(ctx, split.FlowIDs)

	flows := []*Flow{}
	for _, flow := range flowMap {
		flows = append(flows, flow)
	}
	return flows, split, nil
}

func (f *Flow) String() string {
	return fmt.Sprintf("{ID: %s, DataflowRunID: %s, PreviousStepID: %s, NextStepID: %s, State: %s, Message: %s, ContentType: %s, Splits: %v, SplitKey: %s, SplitIndex: %d}",
		f.ID, f.DataflowRunID, f.PreviousStepID, f.NextStepID, f.State, f.Message, f.ContentType, f.Splits, f.SplitKey, f.SplitIndex)
}
