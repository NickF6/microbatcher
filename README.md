# MicroBatcher

MicroBatcher is a lightweight Go library designed to handle micro-batching of jobs. Jobs are batched based on size or a predefined frequency, ensuring efficient processing.

## Features

Batch jobs by size or at regular intervals.
Supports configurable batch sizes and frequencies.
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
	"time"

	"github.com/nickf6/microbatcher"
)

type MyBatchProcessor struct{}

func (bp *MyBatchProcessor) Process(batch []microbatcher.Job) ([]microbatcher.JobResult, error) {
	results := make([]microbatcher.JobResult, len(batch))
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
	processor := &MyBatchProcessor{}
	batcher := microbatcher.NewMicroBatcher(3, 10*time.Millisecond, 3, 100*time.Millisecond, processor, 30)

	// Submit some jobs
	for i := 0; i < 20; i++ {
		resultChan := batcher.Submit(fmt.Sprintf("Job-%d", i))
		go func(resultChan chan microbatcher.JobResult) {
			result := <-resultChan
			fmt.Println(result)
		}(resultChan)
	}

	// Allow jobs to complete
	time.Sleep(300 * time.Millisecond)

	// Shutdown the batcher gracefully
	batcher.Shutdown()
}


```
