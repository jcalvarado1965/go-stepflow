# go-stepflow
Embeddable, scaleout-friendly data flow engine written in Go.

Stepflow lets you define a data flow through a sequence of steps. A data flow starts at an initial step and contains no data. A step transforms the flow (e.g. by modifying the data) before it gets handed off to the next step(s). The transformation depends on the type of step. A data flow can be entirely defined via a JSON document (which can be deserialized into a Dataflow object).

## getting started
The executor engine is instantiated with the NewExecutor function:
```go
func NewExecutor(httpClientFactory HTTPClientFactory, logger Logger, storage Storage, flowQueue FlowQueue) Executor 
```
It expects a number of interfaces to be provided to it, which allows the executor to be embeddable in different environments by providing specific implementations of the interfaces.
1. HTTPClientFactory abstracts the creation of an HTTP client to support environments where this needs to be done outside the built-in packages
1. Logger abstracts message logging
1. Storage abstracts the storage, retrieval and deletion of flow execution objects such as the dataflow run, the steps, split information etc. 
1. FlowQueue abstracts the enqueueing and dequeueing of flows to/from a task queue.

A simple, in-process implementation of these services is provided in the `inprocess` package. See the `main.go` application in `inprocess/cmd` for details on how to instantiate an executor with the in-process implementation, how to deserialize JSON into a Dataflow and how to monitor flow execution. You can run the in-process engine by passing it the path to a dataflow file:
```
go run inprocess/cmd/main.go -dataflow <path-to-dataflow-json>
```
There are several sample dataflow files in the `samples` directory. If you use these samples with the above command you will need to run a web server implementing the endpoints required by the samples (see the `web-method` step type for more information on accessing HTTP endpoints). The node application at https://github.com/jcalvarado1965/node-functions can be to provide the required endpoints.

The `samples` directory has flows demonstrating all the different step types. Step types are described below.

## constant step
This simple flow contains one constant step:

```json
{
   "id": "very-simple-workflow",
   "description": "This only has a constant step",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to the value array",
        "type": "constant",
        "value": [1, 2, 3, 4, 5, 6, 7, 8, 9]
      }
   ]
}
```
The flow has an id and description (both are descriptive only and do not affect execution). The `startAt` property tells the executor which step the flow starts at. The single step in this flow (id `array-constant`) is of type `constant`. A step's `id` must be unique within a flow. A `constant` step transforms the flow by setting the flow data to the contents of the `value` property, an array of integers in this case. Not very useful yet.

## web-method step
The `web-method` is the workhorse of the executor. Most business logic would be executed through a `web-method` step. It takes the flow data and makes an POST HTTP request to a given endpoint, and sets the flow data to the response body. Here is the previous example with an added `web-method` which POSTs the constant array to an endpoint:
```json
{
   "id": "simple-workflow",
   "description": "This has a constant sent to a web method",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to the value array",
        "type": "constant",
        "value": [1, 2, 3, 4, 5, 6, 7, 8, 9],
        "next": "adder"
      },
      {
        "id": "adder",
        "description": "should add the ints",
        "type": "web-method",
        "method": "POST",
        "url": "http://myapp.org/gcf-executor/us-central1/adder"
      }
   ]
}
```
Note the `next` property on the `constant` step which tells the executor which step to execute next.

## distribute and broadcast steps
The executor is scaleout-friendly by providing two step types that split a flow into multiple children flows.

The `distribute` step expects the flow data to be JSON-deserializable into either an array or an object. If the data is an array it creates a child flow for each element of the array, setting their flows to the array element value. If the data is an object it creates a child flow for each key of the object, setting the flow data to the key value. Here is a flow that distributes a 2D array into multiple HTTP requests:
```json
{
   "id": "distributing-workflow",
   "description": "This sends array elements to adder",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to a 2d array",
        "type": "constant",
        "value": [[1, 2, 3], [4, 5, 6], [7, 8, 9]],
        "next": "dist-arrays"
      },
      {
        "id": "dist-arrays",
        "description": "breakout sub arrays",
        "type": "distribute",
        "next": "adder"
      },
      {
        "id": "adder",
        "description": "should add the ints",
        "type": "web-method",
        "method": "POST",
        "url": "http://myapp.org/gcf-executor/us-central1/adder"
      }
   ]
}
```
In this case the `adder` endpoint will be invoked three times, with POST body set to `[1, 2, 3]`, `[4, 5, 6]` and `[7, 8, 9]` respectively.

A broadcast step splits the flow into multiple children based on a list of steps to forward the flow to (i.e. send the flow to multiple steps instead of a single one). Building on the previous example, the following flow distributes a 2d array into two web-method steps:
```json
{
   "id": "distributing-broadcasting-workflow",
   "description": "This sends array elements to adder and multiplier",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to a 2d array",
        "type": "constant",
        "value": [[1, 2, 3], [4, 5, 6], [7, 8, 9]],
        "next": "dist-arrays"
      },
      {
        "id": "dist-arrays",
        "description": "breakout sub arrays",
        "type": "distribute",
        "next": "broadcast-array"
      },
      {
        "id": "broadcast-array",
        "description": "broadcast to add and mult",
        "type": "broadcast",
        "forwardTo": ["adder","multiplier"]
      },
      {
        "id": "adder",
        "description": "should add the ints",
        "type": "web-method",
        "method": "POST",
        "url": "http://myapp.org/gcf-executor/us-central1/adder"
      },
      {
        "id": "multiplier",
        "description": "should multiply the ints",
        "type": "web-method",
        "method": "POST",
        "url": "http://myapp.org/gcf-executor/us-central1/multiplier"
      }
   ]
}
```
In this case both the `adder` and `multiplier` endpoints will be invoked three times, with POST body set to `[1, 2, 3]`, `[4, 5, 6]` and `[7, 8, 9]` respectively.

## join and race steps
Children flows from a split can proceed to completion and the executor will finish the workflow once all children are finished. However it can be useful to merge the children flows before executing subsequent steps (think map/reduce). 

The `join` step will wait until all children flows are finished, then re-activate the parent flow, setting the flow data to the merged data from all the children. If the children were split from a `distribute` step acting on an array, the merged data will be an array containing each of the children flows' data. If the split was from a `distribute` on an object, or a `broadcast` step, the merged data will be an object, with keys set to the `distribute` object keys or the `broadcast` step id's, respectively, and the key values set to the children flows' data.

In the following example, a `distribute` step sends array elements to a web-method, then a join collects the outputs of the web-method requests:
```json
{
   "id": "distributing-and-joining",
   "description": "This distributes to an adder, then join",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to a 2d array",
        "type": "constant",
        "value": [[1, 2, 3], [4, 5, 6], [7, 8, 9]],
        "next": "dist-arrays"
      },
      {
        "id": "dist-arrays",
        "description": "breakout sub arrays",
        "type": "distribute",
        "next": "adder"
      },
      {
        "id": "adder",
        "description": "should add the ints",
        "type": "web-method",
        "method": "POST",
        "url": "http://myapp.org/gcf-executor/us-central1/adder",
        "next": "joiner"
      },
      {
        "id": "joiner",
        "description": "collect adder outputs",
        "type": "join",
        "next": "echo"
      },
      {
        "id": "echo",
        "description": "should echo the joined data",
        "type": "web-method",
        "method": "POST",
        "url": "http://myapp.org/gcf-executor/us-central1/echo"
      },
   ]
}
```
In this case, the final `web-method` step will POST the array `[6, 10, 24]` to the `echo` endpoint.

The `race` step will activate the parent flow as soon as the first non-error child flow arrives. The parent flow's data will be set to the "winnning" childs flow data. All other child flows will be interrupted. If the previous flow is modified to use `race` instead of `join` in the 4th step, the final `web-method` will post either `6`, `10` or `24` to the `echo` endpoint.

# conditional and select steps
A `conditional` step acts like an `if` statement. If the flow data (which must be JSON deserializable) satisfies the given expression, the flow continues to the next step, with its data unchanged. If not, the flow is interrupted. Expressions are evaluated with [github.com/Knetic/govaluate](https://github.com/Knetic/govaluate). Expression variables are represented by jsonpath selectors and evaluated using [github.com/oliveagle/jsonpath](https://github.com/oliveagle/jsonpath). Because jsonpath variables are complex strings they generally need to be enclosed with []. If the jsonpath expression contains [] these need to be further escaped with \\. The following flow demonstrates the behavior of `conditional`:
```json
{
   "id": "conditional-workflow",
   "description": "Demo the conditional step",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to an object array",
        "type": "constant",
        "value": [
          {"name": "joe", "age": 35},
          {"name": "mark", "age": 26},
          {"name": "mary", "age": 42}
          ],
        "next": "dist-arrays"
      },
      {
        "id": "dist-arrays",
        "description": "breakout sub objects",
        "type": "distribute",
        "next": "age-filter"
      },
      {
        "id": "conditional",
        "description": "filter by age",
        "type": "conditional",
        "condition": "[$.age] > 30",
        "next": "echo"
      },
      {
         "id": "echo",
         "description": "call web method echo",
         "type": "web-method",
         "method": "POST",
         "url": "http://localhost:8080/echo"
       }
   ]
}
```
In this example the `echo` endpoint will be called twice, with content `{"name": "joe", "age": 35}` and `{"name": "mary", "age": 42}` respectively.

The `select` step provides simple data manipulation (complex manipulation should be done as business logic via `web-method`). It uses the provided jsonpath expression to select a subset of the incoming data (which must be JSON deserializable). Below is the previous example, modified so that the `echo` endpoint receives only the `name` property.
```json
{
   "id": "conditional-workflow",
   "description": "Demo the conditional step",
   "startAt": "array-constant",
   "steps": [
      {
        "id": "array-constant",
        "description": "sets the flow data to an object array",
        "type": "constant",
        "value": [
          {"name": "joe", "age": 35},
          {"name": "mark", "age": 26},
          {"name": "mary", "age": 42}
          ],
        "next": "dist-arrays"
      },
      {
        "id": "dist-arrays",
        "description": "breakout sub objects",
        "type": "distribute",
        "next": "age-filter"
      },
      {
        "id": "conditional",
        "description": "filter by age",
        "type": "conditional",
        "condition": "[$.age] > 30",
        "next": "select"
      },
      {
        "id": "select",
        "description": "select the name property",
        "type": "select",
        "selector": "$.name",
        "next": "echo"
      },
      {
         "id": "echo",
         "description": "call web method echo",
         "type": "web-method",
         "method": "POST",
         "url": "http://localhost:8080/echo"
       }
   ]
}
```