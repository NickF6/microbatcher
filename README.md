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

func (bp *MyBatchProcessor) Process(batch []microbatcher.Job) []microbatcher.JobResult {
    results := make([]microbatcher.JobResult, len(batch))
    for i, job := range batch {
        fmt.Printf("Processing job: %v\n", job)
        results[i] = fmt.Sprintf("Result for job: %v", job)
    }
    return results
}

func main() {
    processor := &MyBatchProcessor{}
    batcher := microbatcher.NewMicroBatcher(100, 5, 5*time.Second, processor)

    // Submit some jobs
    for i := 0; i < 20; i++ {
        resultChan := batcher.Submit(fmt.Sprintf("Job-%d", i))
        go func(resultChan chan microbatcher.JobResult) {
            result := <-resultChan
            fmt.Println(result)
        }(resultChan)
    }

    // Shutdown the batcher gracefully
    batcher.Shutdown()
}

```
