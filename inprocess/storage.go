package inprocess

import (
	"context"
	"errors"
	"sync"

	stepflow "github.com/jcalvarado1965/go-stepflow"

	"github.com/patrickmn/go-cache"
)

type memoryStorage struct {
	Logger stepflow.Logger
	Cache  *cache.Cache
	mu     sync.Mutex
}

const dataflowRunKind = "DataflowRun:"
const flowKind = "Flow:"
const flowSplitKind = "FlowSplit:"

var errNotFound = errors.New("Not found")

// NewMemoryStorage creates a memory storage service
func NewMemoryStorage(logger stepflow.Logger) stepflow.Storage {
	return &memoryStorage{
		Logger: logger,
		Cache:  cache.New(cache.NoExpiration, cache.NoExpiration),
	}
}

func (ms *memoryStorage) StoreDataflowRun(ctx context.Context, run *stepflow.DataflowRun) error {
	ms.Cache.Set(dataflowRunKind+string(run.ID), run, cache.NoExpiration)
	return nil
}

func (ms *memoryStorage) RetrieveDataflowRuns(ctx context.Context, keys []stepflow.DataflowRunID) map[stepflow.DataflowRunID]*stepflow.DataflowRun {
	runs := make(map[stepflow.DataflowRunID]*stepflow.DataflowRun)
	for _, key := range keys {
		if value, ok := ms.Cache.Get(dataflowRunKind + string(key)); ok {
			runs[key] = value.(*stepflow.DataflowRun)
		} else {
			runs[key] = nil
		}
	}
	return runs
}

func (ms *memoryStorage) DeleteDataflowRun(ctx context.Context, key stepflow.DataflowRunID) error {
	ms.Cache.Delete(dataflowRunKind + string(key))
	return nil
}

func (ms *memoryStorage) StoreFlow(ctx context.Context, flow *stepflow.Flow) error {
	ms.Cache.Set(flowKind+string(flow.ID), flow, cache.NoExpiration)
	return nil
}

func (ms *memoryStorage) RetrieveFlows(ctx context.Context, keys []stepflow.FlowID) map[stepflow.FlowID]*stepflow.Flow {
	flows := make(map[stepflow.FlowID]*stepflow.Flow)
	for _, key := range keys {
		if value, ok := ms.Cache.Get(flowKind + string(key)); ok {
			flows[key] = value.(*stepflow.Flow)
		}
	}
	return flows
}

func (ms *memoryStorage) DeleteFlow(ctx context.Context, key stepflow.FlowID) error {
	ms.Cache.Delete(flowKind + string(key))
	return nil
}

func (ms *memoryStorage) StoreFlowSplit(ctx context.Context, flowSplit *stepflow.FlowSplit) error {
	ms.Cache.Set(flowSplitKind+string(flowSplit.ID), flowSplit, cache.NoExpiration)
	return nil
}

func (ms *memoryStorage) RetrieveFlowSplits(ctx context.Context, keys []stepflow.FlowSplitID) map[stepflow.FlowSplitID]*stepflow.FlowSplit {
	flowSplits := make(map[stepflow.FlowSplitID]*stepflow.FlowSplit)
	for _, key := range keys {
		if value, ok := ms.Cache.Get(flowSplitKind + string(key)); ok {
			flowSplits[key] = value.(*stepflow.FlowSplit)
		}
	}
	return flowSplits
}

func (ms *memoryStorage) DeleteFlowSplit(ctx context.Context, key stepflow.FlowSplitID) error {
	ms.Cache.Delete(flowKind + string(key))
	return nil
}

func (ms *memoryStorage) Increment(ctx context.Context, key string, initialValue int64, increment int64) int64 {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	err := ms.Cache.Add(key, initialValue, cache.NoExpiration)
	if err != nil { // already has value, so increment
		value, _ := ms.Cache.IncrementInt64(key, increment)
		return value
	}

	return initialValue
}

func (ms *memoryStorage) IncrementWithError(ctx context.Context, key string, increment int64, errIncrement int64) (count int64, errCount int64) {
	const errUnit int64 = 1 << 32
	const lowMask int64 = errUnit - 1
	totalIncr := increment + errUnit*errIncrement
	incremented := ms.Increment(ctx, key, totalIncr, totalIncr)
	return incremented & lowMask, incremented / errUnit
}
