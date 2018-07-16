package inprocess

import (
	"context"
	"errors"
	"sync"

	stepflow "github.com/jcalvarado1965/go-stepflow"
)

type memoryQueue struct {
	Logger    stepflow.Logger
	Queue     chan *stepflow.Flow
	IsStopped bool
	DequeueCb func(ctx context.Context, flow *stepflow.Flow) error
	DummyCtx  context.Context
	WaitGroup sync.WaitGroup
}

// NewMemoryQueue creates a memory queue service
func NewMemoryQueue(logger stepflow.Logger, numWorkers int) stepflow.FlowQueue {
	mq := &memoryQueue{
		Logger:    logger,
		Queue:     make(chan *stepflow.Flow, numWorkers),
		DummyCtx:  context.Background(),
		IsStopped: false,
	}

	for i := 0; i < numWorkers; i++ {
		go mq.worker(i)
		mq.WaitGroup.Add(1)
	}
	logger.Debugf(mq.DummyCtx, "Started memory queue with %d workers", numWorkers)
	return mq
}

func (mq *memoryQueue) SetDequeueCb(cb func(ctx context.Context, flow *stepflow.Flow) error) {
	mq.DequeueCb = cb
	mq.Logger.Debugf(mq.DummyCtx, "Dequeue callback set")
}

func (mq *memoryQueue) Enqueue(ctx context.Context, flow *stepflow.Flow) error {
	mq.Logger.Debugf(mq.DummyCtx, "Enqueueing flow %v", flow)
	if mq.IsStopped {
		mq.Logger.Errorf(mq.DummyCtx, "Enqueueing flow on stopped queue %v", flow)
		return errors.New("Queue already stopped")
	}

	mq.Queue <- flow
	return nil
}

func (mq *memoryQueue) Stop(ctx context.Context) (*sync.WaitGroup, error) {
	mq.Logger.Infof(mq.DummyCtx, "Stopping memory queue")
	if mq.IsStopped {
		return nil, errors.New("Queue already stopped")
	}

	mq.IsStopped = true
	close(mq.Queue)
	return &mq.WaitGroup, nil
}

func (mq *memoryQueue) worker(workerID int) {
	mq.Logger.Infof(mq.DummyCtx, "Worker %d: starting...", workerID)
	for flow := range mq.Queue {
		mq.Logger.Debugf(mq.DummyCtx, "Worker %d: dequeued flow %v", workerID, flow)
		if mq.DequeueCb != nil {
			mq.DequeueCb(mq.DummyCtx, flow)
		} else {
			mq.Logger.Errorf(mq.DummyCtx, "Worker %d: no callback for dequeued flow %v", workerID, flow)
		}
	}
	mq.Logger.Infof(mq.DummyCtx, "Worker %d: exiting...", workerID)
	mq.WaitGroup.Done()
}
