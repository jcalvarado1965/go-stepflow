package stepflow

import (
	"context"
	"net/http"
)

// FlowContextKeyType used to store flow id in context
type FlowContextKeyType struct{}

// DataflowRunContextKeyType used to store workflow run id in context
type DataflowRunContextKeyType struct{}

// StepContextKeyType used to store step ID in context
type StepContextKeyType struct{}

// Used to store info in context
var (
	FlowContextKey        = FlowContextKeyType{}
	DataflowRunContextKey = DataflowRunContextKeyType{}
	StepContextKey        = StepContextKeyType{}
)

// Executor is the interface implemented by the executing engine
type Executor interface {
	Start(ctx context.Context, workflow *Dataflow) (*DataflowRun, []error)
	Validate(ctx context.Context, workflow *Dataflow) []error
	Interrupt(ctx context.Context, run *DataflowRun)

	GetHTTPClientFactory() HTTPClientFactory
	GetLogger() Logger
	GetStorage() Storage
}

// Storage is the interface implemented by external storage service
type Storage interface {
	StoreDataflowRun(ctx context.Context, run *DataflowRun) error
	RetrieveDataflowRuns(ctx context.Context, keys []DataflowRunID) map[DataflowRunID]*DataflowRun
	DeleteDataflowRun(ctx context.Context, key DataflowRunID) error

	StoreFlow(ctx context.Context, flow *Flow) error
	RetrieveFlows(ctx context.Context, keys []FlowID) map[FlowID]*Flow
	DeleteFlow(ctx context.Context, key FlowID) error

	StoreFlowSplit(ctx context.Context, flowSplit *FlowSplit) error
	RetrieveFlowSplits(ctx context.Context, keys []FlowSplitID) map[FlowSplitID]*FlowSplit
	DeleteFlowSplit(ctx context.Context, key FlowSplitID) error

	Increment(ctx context.Context, key string, initialValue int64, increment int64) int64
	IncrementWithError(ctx context.Context, key string, increment int64, errIncrement int64) (count int64, errCount int64)
}

// FlowQueue is the interface implemented by external queue service
type FlowQueue interface {
	SetDequeueCb(func(ctx context.Context, flow *Flow) error)
	Enqueue(ctx context.Context, flow *Flow) error
}

// Logger is passed to other services for pluggable logging
type Logger interface {
	Debugf(ctx context.Context, fmt string, params ...interface{})
	Infof(ctx context.Context, fmt string, params ...interface{})
	Warnf(ctx context.Context, fmt string, params ...interface{})
	Errorf(ctx context.Context, fmt string, params ...interface{})
}

// HTTPClientFactory is used to abstract HTTP client creation
type HTTPClientFactory interface {
	GetHTTPClient(ctx context.Context, disableSSLValidation bool) *http.Client
}
