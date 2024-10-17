# MicroBatcher

MicroBatcher is a lightweight Go library designed to handle micro-batching of jobs. Jobs are batched based on size or a predefined frequency, ensuring efficient processing.

## Features

Batch jobs by size or at regular intervals.
Supports configurable batch sizes and frequencies.
Supports error handling including retry counts, and retry delay.
Graceful shutdown ensuring all jobs are processed.
Fully concurrent and safe for use in multi-threaded applications.

## Installation

To install the MicroBatcher package, use the following command:

`go get github.com/nickf6/microbatcher`

## Testing

To run tests, use the go test command:

`go test`

or for a verbose output

`go test -v`

## Example

Here’s a detailed example demonstrating how to integrate MicroBatcher into a Go program:

```
package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/nickf6/microbatcher"
)

type MyBatchProcessor struct{}

func failedToProcess() bool {
	return rand.Intn(100) < 33
}

func (bp *MyBatchProcessor) Process(batch []microbatcher.Job) ([]microbatcher.JobResult, error) {
	results := make([]microbatcher.JobResult, len(batch))
	if failedToProcess() {
		return nil, fmt.Errorf("batch failed to process")
	}
	for i, job := range batch {
		if i == len(batch)-1 {
			fmt.Printf("Processing job: %v -- end of batch -- \n", job)
		} else {
			fmt.Printf("Processing job: %v\n", job)
		}

		results[i] = fmt.Sprintf("Result for job: %v", job)
	}
	return results, nil

}

func main() {
	batchSize := 3

	processor := &MyBatchProcessor{}
	batcher := microbatcher.NewMicroBatcher(batchSize, 10*time.Millisecond, 3, 100*time.Millisecond, processor, 30)

	results := make([]chan microbatcher.JobResult, 20)

	// Submit jobs
	for i := 0; i < 20; i++ {
		results[i] = batcher.Submit(fmt.Sprintf("Job-%d", i))
	}

	// Shutdown the batcher
	batcher.Shutdown()

	// Print results and any batch that failed to process
	for _, result := range results {
		result := <-result
		err, ok := result.(error)
		if ok {
			fmt.Println("Error occured:", err)
		}
		fmt.Println(result)
	}
}

```
