package stepflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type executor struct {
	HTTPClientFactory HTTPClientFactory
	Logger            Logger
	Storage           Storage
	FlowQueue         FlowQueue
}

// NewExecutor creates an instance of the execution engine
func NewExecutor(httpClientFactory HTTPClientFactory, logger Logger, storage Storage, flowQueue FlowQueue) Executor {
	e := &executor{
		Logger:            logger,
		Storage:           storage,
		FlowQueue:         flowQueue,
		HTTPClientFactory: httpClientFactory,
	}

	flowQueue.SetDequeueCb(e.handleFlow)

	return e
}

func (e *executor) Validate(ctx context.Context, workflow *Dataflow) []error {
	errs := []error{}
	if workflow == nil {
		errs = append(errs, errors.New("Dataflow is nil"))
	} else {
		steps := workflow.Steps
		if len(steps) == 0 {
			errs = append(errs, errors.New("Dataflow has no steps"))
		}
		for _, step := range steps {
			errs = append(errs, step.Validate()...)
		}
	}
	return errs
}

func (e *executor) Start(ctx context.Context, workflow *Dataflow) (*DataflowRun, []error) {
	errs := e.Validate(ctx, workflow)
	if len(errs) > 0 {
		return nil, errs
	}

	e.Logger.Debugf(ctx, "Dataflow %v started", *workflow)

	wr := NewDataflowRun(workflow)

	err := e.Storage.StoreDataflowRun(ctx, wr)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	// create initial flow
	flow := Flow{
		FlowNoData: FlowNoData{
			ID:            FlowID(uuid.New().String()),
			DataflowRunID: (*wr).ID,
			NextStepID:    (*workflow).StartAt.GetID(),
			State:         FlowStateActive,
		},
	}

	err = e.enqueueFlow(ctx, &flow)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	return wr, errs
}

// Interrupt marks the workflow run as interrupted so that future flows
// being dequeued will result in no action
func (e *executor) Interrupt(ctx context.Context, run *DataflowRun) {

}

func (e *executor) GetLogger() Logger {
	return e.Logger
}

func (e *executor) GetStorage() Storage {
	return e.Storage
}

func (e *executor) GetHTTPClientFactory() HTTPClientFactory {
	return e.HTTPClientFactory
}

func (e *executor) handleFlow(ctx context.Context, flow *Flow) error {
	var err error
	var dfctx = context.WithValue(ctx, FlowContextKey, flow.ID)
	dfctx = context.WithValue(dfctx, DataflowRunContextKey, flow.DataflowRunID)
	e.Logger.Infof(dfctx, "Executor received flow %s", flow)
	if run, ok := e.Storage.RetrieveDataflowRuns(dfctx, []DataflowRunID{flow.DataflowRunID})[flow.DataflowRunID]; ok {
		dfctx = context.WithValue(dfctx, StepContextKey, flow.NextStepID)
		step := run.Dataflow.GetStep(flow.NextStepID)
		if step == nil {
			err = fmt.Errorf("step not found in workflow")
		} else {
			if run.State != RunStateActive {
				run.State = RunStateActive
				e.Storage.StoreDataflowRun(dfctx, run)
			}
			switch s := step.(type) {
			case DoerStep:
				e.Logger.Debugf(dfctx, "Executor calling Do")
				if err = s.Do(dfctx, e, flow); err == nil {
					err = e.advanceFlow(ctx, run, flow, step)
				} else {
					e.Logger.Errorf(dfctx, "Error doing step: %s", err.Error())
					e.failFlow(dfctx, run, flow, step)
				}
			case SplitterStep:
				e.Logger.Debugf(dfctx, "Executor calling Split")
				if flows, split, err := s.Split(dfctx, e, flow); err == nil {
					e.Storage.StoreFlowSplit(ctx, split)
					flow.State = FlowStateSplit
					e.Storage.StoreFlow(ctx, flow)
					e.GetLogger().Infof(ctx, "Flow %s split into %v", flow.ID, split)
					for _, f := range flows {
						// splitter steps are expected to set the next step id of
						// the children flows
						err = e.advanceFlow(ctx, run, f, nil)
						if err != nil {
							break
						}
					}
				} else {
					e.Logger.Errorf(dfctx, "Error splitting step: %s", err.Error())
					e.failFlow(dfctx, run, flow, step)
				}
			case JoinerStep:
				e.Logger.Debugf(dfctx, "Executor calling Join")
				if joinedFlow, err := s.Join(dfctx, e, flow); joinedFlow != nil {
					// flows are joined, so clean up
					if siblings, _, err := flow.getSiblingFlows(ctx, e); err == nil {
						for _, sibling := range siblings {
							if sibling.State != FlowStateError {
								e.Storage.DeleteFlow(ctx, sibling.ID)
							}
						}
					}
					if err == nil {
						err = e.advanceFlow(ctx, run, joinedFlow, step)
					} else {
						// join had one or more input flow errors
						e.Logger.Errorf(dfctx, "Error doing step: %s", err.Error())
						e.failFlow(dfctx, run, joinedFlow, step)
					}
				}
			default:
				err = fmt.Errorf("Step does not support execution")
			}
		}
		if err != nil {
			e.Logger.Errorf(dfctx, "Step failed: %s", err.Error())
		}
	} else {
		err = fmt.Errorf("Dataflow run not found")
		e.Logger.Errorf(dfctx, err.Error())
	}

	return err
}

// when a flow finishes (completes or errors) this method calculates the
// new workflow state. If the flow is the root, the workflow is finished.
// Otherwise recursively calculates the state of the ancestor flows by
// getting the siblings state, updating the parent flow and repeating if needed.
// Flows are deleted on normal completion, so siblings are finished if
// not found, or if all in error.
func (e *executor) updateDataflowState(ctx context.Context, run *DataflowRun, flow *Flow, step Step) error {
	currFlow := flow
	isDataflowError := false

	// if the current (finished) flow will reach a joining step, do not handle it here
	// since the joining step needs to see the flow (e.g. to determine when all
	// children flows are done)
	nextID := step.GetNextID()
	for ; nextID != ""; nextID = step.GetNextID() {
		step = run.Dataflow.GetStep(nextID)
		if _, ok := step.(JoinerStep); ok {
			e.Logger.Debugf(ctx, "Flow advanced to step %s", step.GetID())
			flow.NextStepID = step.GetID()
			if err := e.Storage.StoreFlow(ctx, flow); err != nil {
				return err
			}
			err := e.enqueueFlow(ctx, flow)
			return err
		}
	}

	// while current flow is not the root
	for len(currFlow.Splits) > 0 {
		split, err := currFlow.getLastSplit(ctx, e)
		if err != nil {
			return err
		}

		var errIncr int64
		if flow.State == FlowStateError {
			errIncr = 1
		}
		totalFinish, totalError := e.GetStorage().IncrementWithError(ctx, string(split.ID), 1, errIncr)

		if totalFinish < int64(len(split.FlowIDs)) {
			return nil // not all siblings are finished
		}

		e.GetLogger().Infof(ctx, "All children flows of %s are finished (%d with error)", split.ParentFlowID, totalError)
		if totalError > 0 {
			isDataflowError = true
		}

		// all siblings are finished, so update parent flow and repeat
		parentFlow, ok := e.Storage.RetrieveFlows(ctx, []FlowID{split.ParentFlowID})[split.ParentFlowID]
		if !ok {
			return fmt.Errorf("Flow with ID %s not found", split.ParentFlowID)
		}

		currFlow = parentFlow

		if totalError > 0 { // at least one error
			e.GetLogger().Warnf(ctx, "Setting %s state to error", split.ParentFlowID)
			currFlow.State = FlowStateError
			if err = e.Storage.StoreFlow(ctx, currFlow); err != nil {
				return err
			}
		} else {
			e.GetLogger().Infof(ctx, "Setting %s state to completed", split.ParentFlowID)
			currFlow.State = FlowStateCompleted
			if err = e.Storage.DeleteFlow(ctx, currFlow.ID); err != nil {
				return err
			}
		}
	}

	// if we got this far the workflow is finished
	if isDataflowError {
		e.GetLogger().Warnf(ctx, "Dataflow %s completed with error", run.ID)
		run.State = RunStateError
	} else {
		e.GetLogger().Infof(ctx, "Dataflow %s completed successfully", run.ID)
		run.State = RunStateCompleted
	}

	return e.Storage.StoreDataflowRun(ctx, run)
}

func (e *executor) advanceFlow(ctx context.Context, run *DataflowRun, flow *Flow, step Step) (err error) {
	if flow.State == FlowStateInterrupted {
		// flow can be interrupted e.g. by a conditional. in that case
		// we don't need to advance it, but we need to update workflow state
		// as it should be treated as a non-error completion
		err := e.Storage.StoreFlow(ctx, flow)
		if err == nil {
			err = e.updateDataflowState(ctx, run, flow, step)
		}
		return err
	}

	if step != nil {
		flow.PreviousStepID = step.GetID()
		flow.NextStepID = step.GetNextID()
	} // else the flow is already setup for next step

	if flow.NextStepID != "" {
		err = e.enqueueFlow(ctx, flow)
	} else {
		flow.State = FlowStateCompleted
		if err = e.Storage.DeleteFlow(ctx, flow.ID); err != nil {
			return err
		}
		err = e.updateDataflowState(ctx, run, flow, step)
	}

	return err
}

func (e *executor) enqueueFlow(ctx context.Context, flow *Flow) error {
	err := e.Storage.StoreFlow(ctx, flow)
	if err == nil {
		err = e.FlowQueue.Enqueue(ctx, flow)
		if err != nil {
			e.Logger.Errorf(ctx, "Error enqueuing flow %s: %s", flow, err.Error())
			e.Storage.DeleteFlow(ctx, flow.ID)
		}
	}

	return err
}

func (e *executor) failFlow(ctx context.Context, run *DataflowRun, flow *Flow, step Step) error {
	var err error
	flow.State = FlowStateError
	if err = e.Storage.StoreFlow(ctx, flow); err != nil {
		return err
	}

	return e.updateDataflowState(ctx, run, flow, step)
}
