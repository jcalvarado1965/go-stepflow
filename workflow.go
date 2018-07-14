package stepflow

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// const WorkflowRunKind = "DataflowRun"

// Dataflow defines a workflow
type Dataflow struct {
	ID          string          `json:"id,omitempty"`
	Description string          `json:"description,omitempty"`
	Steps       []Step          `json:"-"`
	StartAt     Step            `json:"-"`
	StepMap     map[string]Step `json:"-"`
}

// GetStep returns the workflow step with the given ID
func (w *Dataflow) GetStep(ID string) Step {
	if w.StepMap == nil {
		for _, step := range w.Steps {
			w.StepMap[step.GetID()] = step
		}
	}

	if step, ok := w.StepMap[ID]; ok {
		return step
	}

	return nil
}

// WorkflowNoFn erases the marshaling functions to avoid recursion
type WorkflowNoFn Dataflow

// WorkflowMarshaller is used for marshaling a workflow into JSON
type WorkflowMarshaller struct {
	WorkflowNoFn
	StepsJSON []json.RawMessage `json:"steps"`
	StartID   string            `json:"startAt,omitempty"`
}

// MarshalJSON implements Marshaller for Dataflow
func (w Dataflow) MarshalJSON() ([]byte, error) {
	dfNoMar := WorkflowMarshaller{WorkflowNoFn: WorkflowNoFn(w)}
	// dfNoMar.ID = w.ID
	// dfNoMar.Description = w.Description

	for _, step := range dfNoMar.Steps {
		step.PrepareMarshal()
		stepJSON, err := json.Marshal(step)
		if err != nil {
			return nil, err
		}
		dfNoMar.StepsJSON = append(dfNoMar.StepsJSON, stepJSON)
	}
	if dfNoMar.StartAt != nil {
		dfNoMar.StartID = dfNoMar.StartAt.GetID()
	}

	return json.Marshal(dfNoMar)
}

// UnmarshalJSON implements Unmarshaller for Dataflow
func (w *Dataflow) UnmarshalJSON(bytes []byte) error {
	var dfNoMar WorkflowMarshaller
	if err := json.Unmarshal(bytes, &dfNoMar); err != nil {
		return err
	}

	stepMap := map[string]Step{}
	// w.ID = dfNoMar.ID
	// w.Description = dfNoMar.Description
	for _, raw := range dfNoMar.StepsJSON {
		step, err := UnmarshalStep(raw)
		if err != nil {
			return err
		}
		dfNoMar.Steps = append(dfNoMar.Steps, step)
		stepMap[step.GetID()] = step
	}

	for _, step := range dfNoMar.Steps {
		if err := step.ResolveIDs(stepMap); err != nil {
			return err
		}
	}

	if dfNoMar.StartID != "" {
		var ok bool
		dfNoMar.StartAt, ok = stepMap[dfNoMar.StartID]
		if !ok {
			return fmt.Errorf("Start StepID %s not found in workflow", dfNoMar.StartID)
		}
	}

	dfNoMar.StepMap = stepMap
	*w = Dataflow(dfNoMar.WorkflowNoFn)
	return nil
}

func (w Dataflow) String() string {
	var startAt string
	if w.StartAt != nil {
		startAt = w.StartAt.GetID()
	}
	return fmt.Sprintf("{ID: %s, Desc: %s, Steps: %s, Start: %s}",
		w.ID, w.Description, w.Steps, startAt)
}

// WorkflowRunID identifies a workflow run (generated UUID)
type WorkflowRunID string

// WorkflowRunState is the string type of workflow run states
type WorkflowRunState string

// list of workflow run states
const (
	RunStateNew         = WorkflowRunState("New")
	RunStateActive      = WorkflowRunState("Active")
	RunStateInterrupted = WorkflowRunState("Interrupted")
	RunStateCompleted   = WorkflowRunState("Completed")
	RunStateError       = WorkflowRunState("Error")
)

// DataflowRun describes a running workflow
type DataflowRun struct {
	ID       WorkflowRunID
	Dataflow *Dataflow
	State    WorkflowRunState
}

// NewWorkflowRun creates a run
func NewWorkflowRun(df *Dataflow) *DataflowRun {
	return &DataflowRun{
		ID:       WorkflowRunID(uuid.New().String()),
		Dataflow: df,
		State:    RunStateNew,
	}
	// rawJSON, err := json.Marshal(df)

	// if err != nil {
	// 	return nil, err
	// }

	// return &DataflowRun{
	// 	WorkflowID: df.ID,
	// 	Dataflow:   rawJSON}, nil
}

// StoreRun creates the datastore entities for the run
// func (w Dataflow) StoreRun(ctx context.Context) error {
// 	wr, _ := NewWorkflowRun(&w)
// 	return datastore.RunInTransaction(ctx, func(tc xnc.Context) error {
// 		wRunKey, err := datastore.Put(tc, datastore.NewIncompleteKey(tc, WorkflowRunKind, nil), wr)
// 		if err != nil {
// 			return err
// 		}

// 		for _, step := range w.Steps {
// 			stepRun := &StepRun{
// 				StepID:        step.GetID(),
// 				WorkflowRunID: wRunKey.IntID()}
// 			key := stepRun.GetDatastoreKey(tc)
// 			_, err = datastore.Put(tc, key, stepRun)
// 			if err != nil {
// 				return err
// 			}
// 		}

// 		return nil
// 	}, &datastore.TransactionOptions{XG: true})
// }
