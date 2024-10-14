package microbatcher

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockResult struct {
	batch int
	index int
}

// String returns a formatted string representation of the mockResult.
func (m mockResult) String() string {
	return fmt.Sprintf("Batch: %d, Index: %d", m.batch, m.index)
}

type mockProcessor struct {
	batches [][]Job
}

// Mock processing a batch of jobs and returning a slice of job completion statuses as strings.
func (mp *mockProcessor) Process(batch []Job) ([]JobResult, error) {
	jobResults := make([]JobResult, len(batch))
	for i := range batch {
		jobResult := mockResult{batch: len(mp.batches), index: i}
		jobResults[i] = jobResult
	}
	mp.batches = append(mp.batches, batch)
	return jobResults, nil
}

// TestMicroBatcherCompleteBatches tests the micro-batcher functionality when processing complete batches of jobs.
func TestMicroBatcherCompleteBatches(t *testing.T) {

	const totalJobs = 30
	const batchSize = 5
	const batchFreq = 50 * time.Millisecond
	const maxRetries = 3
	const retryInterval = 100 * time.Millisecond

	processor := &mockProcessor{}
	mb := NewMicroBatcher(
		batchSize,
		batchFreq,
		maxRetries,
		retryInterval,
		processor,
		totalJobs,
	)

	results := make([]chan JobResult, totalJobs)

	for i := 0; i < totalJobs; i++ {
		results[i] = make(chan JobResult)
		results[i] = mb.Submit(i)
	}

	batchRuns := (totalJobs + batchSize - 1) / batchSize

	for i := 0; i < batchRuns; i++ {
		for j := 0; j < batchSize; j++ {
			jobResult := mockResult{batch: i, index: j}
			runIndex := i*batchSize + j

			if runIndex < totalJobs {
				res := <-results[runIndex]
				mockRes, ok := res.(mockResult)

				assert.Equal(t, true, ok, fmt.Sprintf("Type assert error for %s", jobResult.String()))
				assert.Equal(t, i, mockRes.batch, fmt.Sprintf("Batch index error for %s", jobResult.String()))
				assert.Equal(t, j, mockRes.index, fmt.Sprintf("Job index error for %s", jobResult.String()))
			}
		}
	}
}

// TestMicroBatcherWithPartialBatch tests the micro-batcher functionality when processing jobs where the final batch is incomplete.
func TestMicroBatcherWithPartialBatch(t *testing.T) {

	const totalJobs = 28
	const batchSize = 5
	const batchFreq = 50 * time.Millisecond
	const maxRetries = 3
	const retryInterval = 100 * time.Millisecond

	processor := &mockProcessor{}
	mb := NewMicroBatcher(
		batchSize,
		batchFreq,
		maxRetries,
		retryInterval,
		processor,
		totalJobs,
	)

	results := make([]chan JobResult, totalJobs)

	for i := 0; i < totalJobs; i++ {
		results[i] = make(chan JobResult)
		results[i] = mb.Submit(i)
	}

	batchRuns := (totalJobs + batchSize - 1) / batchSize

	for i := 0; i < batchRuns; i++ {
		for j := 0; j < batchSize; j++ {
			jobResult := mockResult{batch: i, index: j}
			runIndex := i*batchSize + j

			if runIndex < totalJobs {
				res := <-results[runIndex]
				mockRes, ok := res.(mockResult)

				assert.Equal(t, true, ok, fmt.Sprintf("Type assert error for %s", jobResult.String()))
				assert.Equal(t, i, mockRes.batch, fmt.Sprintf("Batch index error for %s", jobResult.String()))
				assert.Equal(t, j, mockRes.index, fmt.Sprintf("Job index error for %s", jobResult.String()))
			}
		}
	}
}

// TestMicroBatcherShutdown tests that jobs submitted to the micro-batcher are properly resolved after Shutdown is called.
func TestMicroBatcherShutdown(t *testing.T) {
	const totalJobs = 7
	const batchSize = 3
	const batchFreq = 100 * time.Millisecond
	const maxRetries = 3
	const retryInterval = 100 * time.Millisecond

	processor := &mockProcessor{}
	mb := NewMicroBatcher(
		batchSize,
		batchFreq,
		maxRetries,
		retryInterval,
		processor,
		totalJobs,
	)

	results := make([]chan JobResult, totalJobs)

	// Submit jobs
	for i := 0; i < totalJobs; i++ {
		results[i] = make(chan JobResult)
		results[i] = mb.Submit(i)
	}

	mb.Shutdown()

	for i := 0; i < totalJobs; i++ {
		select {
		case <-results[i]: // Blocking if no value is available
			continue
		default: // This executes if no value is available
			t.Errorf("Result %d not resolved after Shutdown() executed at index: ", i)
		}
	}
}

// mockProcessorWithRetries is a mock batch processor that fails a configurable number of times before succeeding.
type mockProcessorWithRetries struct {
	batches               [][]Job
	failuresBeforeSuccess int
	callCount             int
}

// Process simulates failure before succeeding after a given number of attempts.
func (mp *mockProcessorWithRetries) Process(batch []Job) ([]JobResult, error) {
	mp.callCount++

	if mp.callCount <= mp.failuresBeforeSuccess {
		return nil, fmt.Errorf("simulated batch failure on attempt %d", mp.callCount)
	}

	jobResults := make([]JobResult, len(batch))
	for i := range batch {
		jobResult := mockResult{batch: len(mp.batches), index: i}
		jobResults[i] = jobResult
	}
	mp.batches = append(mp.batches, batch)
	return jobResults, nil
}

// TestMicroBatcherErrorHandlingWithRetries simulates a processor that fails initially and succeeds after retries.
func TestMicroBatcherErrorHandlingWithRetries(t *testing.T) {

	const totalJobs = 5
	const batchSize = 5
	const batchFreq = 50 * time.Millisecond
	const maxRetries = 3
	const retryInterval = 100 * time.Millisecond

	processor := &mockProcessorWithRetries{
		failuresBeforeSuccess: 2,
	}

	mb := NewMicroBatcher(
		batchSize,
		batchFreq,
		maxRetries,
		retryInterval,
		processor,
		totalJobs,
	)

	results := make([]chan JobResult, totalJobs)

	for i := 0; i < totalJobs; i++ {
		results[i] = make(chan JobResult)
		results[i] = mb.Submit(i)
	}

	// Expect all jobs to succeed after retries
	for i := 0; i < totalJobs; i++ {
		res := <-results[i]
		mockRes, ok := res.(mockResult)
		assert.True(t, ok, "Expected job result to be mockResult")
		assert.Equal(t, i, mockRes.index, "Unexpected job index")
	}
}

// TestMicroBatcherErrorHandlingMaxRetriesExceeded simulates a processor that always fails and exceeds max retries.
func TestMicroBatcherErrorHandlingMaxRetriesExceeded(t *testing.T) {

	const totalJobs = 5
	const batchSize = 5
	const batchFreq = 50 * time.Millisecond
	const maxRetries = 3
	const retryInterval = 100 * time.Millisecond

	processor := &mockProcessorWithRetries{
		failuresBeforeSuccess: math.MaxInt32, // Ensure it never succeeds
	}

	mb := NewMicroBatcher(
		batchSize,
		batchFreq,
		maxRetries,
		retryInterval,
		processor,
		totalJobs,
	)

	results := make([]chan JobResult, totalJobs)

	for i := 0; i < totalJobs; i++ {
		results[i] = make(chan JobResult)
		results[i] = mb.Submit(i)
	}

	// Expect all jobs to fail after max retries
	for i := 0; i < totalJobs; i++ {
		res := <-results[i]
		_, ok := res.(error) // Expect an error type
		assert.True(t, ok, "Expected an error after max retries")
	}
}
