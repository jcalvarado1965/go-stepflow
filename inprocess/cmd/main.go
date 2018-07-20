package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	stepflow "github.com/jcalvarado1965/go-stepflow"
	"github.com/jcalvarado1965/go-stepflow/inprocess"
)

func main() {
	wfFile := flag.String("dataflow", "", "Path to JSON-serialized workflow")
	flag.Parse()
	if *wfFile == "" {
		flag.PrintDefaults()
		return
	}

	bytes, err := ioutil.ReadFile(*wfFile)
	if err != nil {
		fmt.Printf("File %s could not be read: %s\n", *wfFile, err.Error())
		return
	}

	var workflow stepflow.Dataflow
	err = json.Unmarshal(bytes, &workflow)
	if err != nil {
		fmt.Printf("Dataflow in %s could not be read: %s\n", *wfFile, err.Error())
		return
	}

	httpClientFactory := inprocess.NewHTTPClientFactory()
	logger := inprocess.NewConsoleLogger()
	flowQueue := inprocess.NewMemoryQueue(logger, 10)
	storage := inprocess.NewMemoryStorage(logger)
	executor := stepflow.NewExecutor(httpClientFactory, logger, storage, flowQueue)
	ctx := context.Background()

	run, errs := executor.Start(ctx, &workflow)

	if len(errs) > 0 {
		for _, err := range errs {
			logger.Errorf(ctx, err.Error())
		}
		return
	}

	for {
		time.Sleep(time.Second)
		run, ok := storage.RetrieveDataflowRuns(ctx, []stepflow.DataflowRunID{run.ID})[run.ID]
		if !ok ||
			run.State == stepflow.RunStateCompleted ||
			run.State == stepflow.RunStateError ||
			run.State == stepflow.RunStateInterrupted {
			break
		}
	}

	wg, _ := flowQueue.(*inprocess.MemoryQueue).Stop(ctx)

	if wg != nil {
		logger.Debugf(ctx, "Waiting for queue to stop")
		wg.Wait()
	}
}
