package microbatcher

import (
	"log"
	"math"
	"sync"
	"time"
)

// Job represents a single unit of work to be processed.
type Job interface{}

// JobResult represents the result of a processed job.
type JobResult interface{}

// BatchProcessor is an interface for processing a batch of jobs.
// This should be implemented by the user to define how jobs are processed.
type BatchProcessor interface {
	// Process processes a batch of jobs and returns their results.
	Process(batch []Job) ([]JobResult, error)
}

// MicroBatcher is a struct that handles batching of jobs, ensuring that they are
// processed either when a batch size limit is reached or after a given frequency interval.
type MicroBatcher struct {
	batchSize          int                    // Size of each batch to be processed.
	batchFreq          time.Duration          // Frequency at which to process batches.
	maxRetries         int                    // Maximum number of retries if batch process returns an error.
	retryInterval      time.Duration          // Initial retry interval for backoff after batch error.
	processor          BatchProcessor         // The processor responsible for handling batch processing.
	jobs               []Job                  // List of currently queued jobs.
	JobToResultChannel map[Job]chan JobResult // Map of jobs to their result channels.
	mutex              sync.Mutex             // Mutex to ensure safe access to shared resources.
	addChan            chan Job               // Channel for adding jobs to the queue.
	batchChan          chan []Job             // Channel to send batches of jobs for processing.
	notifyChan         chan struct{}          // Channel to notify when a job is added.
	additionsWG        sync.WaitGroup         // WaitGroup for tracking job additions.
	batchesWG          sync.WaitGroup         // WaitGroup for tracking batch processing.
	jobsWG             sync.WaitGroup         // WaitGroup for tracking job results processing.
}

// NewMicroBatcher creates and initializes a new MicroBatcher instance.
// It starts the necessary goroutines for handling job additions, batch processing,
// and job result processing.
func NewMicroBatcher(
	batchSize int,
	batchFreq time.Duration,
	maxRetries int,
	retryInterval time.Duration,
	processor BatchProcessor,
	jobChannelBuffer int,
) *MicroBatcher {

	mb := &MicroBatcher{
		batchSize:          batchSize,
		batchFreq:          batchFreq,
		maxRetries:         maxRetries,
		retryInterval:      retryInterval,
		processor:          processor,
		jobs:               make([]Job, 0, batchSize),
		addChan:            make(chan Job, jobChannelBuffer),
		batchChan:          make(chan []Job, jobChannelBuffer),
		notifyChan:         make(chan struct{}, jobChannelBuffer),
		JobToResultChannel: make(map[Job]chan JobResult),
	}

	// Start the goroutines for job, batch, and addition processing
	mb.jobsWG.Add(1)
	go mb.processJobs()

	mb.batchesWG.Add(1)
	go mb.processBatches()

	mb.additionsWG.Add(1)
	go mb.processAdditions()

	return mb
}

// Submit adds a job to the MicroBatcher and returns a channel through which the result of the job will be communicated.
func (mb *MicroBatcher) Submit(job Job) chan JobResult {
	resultChan := make(chan JobResult, 1)

	// Add job to the processing queue
	mb.mutex.Lock()
	mb.JobToResultChannel[job] = resultChan
	mb.mutex.Unlock()

	mb.addChan <- job

	return resultChan
}

// processAddition handles the addition of a job to the batch.
func (mb *MicroBatcher) processAddition(job Job) {
	// Add the job to the current batch
	mb.mutex.Lock()
	mb.jobs = append(mb.jobs, job)
	mb.mutex.Unlock()

	// Notify that a job has been added
	mb.notifyChan <- struct{}{}
}

// processAdditions continuously listens for new jobs to add to the batch.
func (mb *MicroBatcher) processAdditions() {
	defer mb.additionsWG.Done() // Signal that job addition processing is done when this exits

	for {
		job, ok := <-mb.addChan
		if !ok {
			return
		}
		mb.processAddition(job)
	}
}

// processCompleteBatch sends a full batch of jobs to the batch processing channel.
func (mb *MicroBatcher) processCompleteBatch() {
	batch := mb.jobs[:mb.batchSize]
	mb.jobs = mb.jobs[mb.batchSize:]
	mb.mutex.Unlock()
	mb.batchChan <- batch
}

// processPartialBatch sends a partial batch of jobs to the batch processing channel.
func (mb *MicroBatcher) processPartialBatch() {
	batch := mb.jobs
	mb.jobs = []Job{}
	mb.mutex.Unlock()
	mb.batchChan <- batch
}

// processBatches listens for new jobs and processes them either when a batch is full or at a set time interval.
func (mb *MicroBatcher) processBatches() {
	defer mb.batchesWG.Done() // Signal that batch processing is done when this exits

	for {
		timer := time.After(mb.batchFreq)
		select {
		case _, ok := <-mb.notifyChan:
			if !ok {
				if len(mb.jobs) > 0 {
					mb.mutex.Lock()
					mb.processPartialBatch()
				}
				return
			}
			mb.mutex.Lock()
			if len(mb.jobs) >= mb.batchSize {
				mb.processCompleteBatch()
			} else {
				mb.mutex.Unlock()
			}
		case <-timer:
			mb.mutex.Lock()
			if len(mb.jobs) > 0 {
				mb.processPartialBatch()
			} else {
				mb.mutex.Unlock()
			}
		}
	}
}

// processJob processes a single batch of jobs and sends the results back to their respective channels.
func (mb *MicroBatcher) processJob(batch []Job, attempt int) {
	results, err := mb.processor.Process(batch)

	if err != nil {
		if attempt <= mb.maxRetries {
			// Calculate exponential backoff delay
			backoffDuration := mb.retryInterval * time.Duration(math.Pow(2, float64(attempt)))
			log.Printf("Error encountered: %v. Retrying batch in %v (attempt %d/%d)", err, backoffDuration, attempt, mb.maxRetries)

			time.Sleep(backoffDuration) // Wait for backoff duration before retrying
			mb.processJob(batch, attempt+1)
		} else {
			log.Printf("Batch processing failed after %d attempts: %v", attempt, err)
			// Optionally, you can send error to result channels or handle it in some other way
			mb.failBatch(batch, err)
		}
		return
	}

	mb.mapJobResults(batch, results)
}

func (mb *MicroBatcher) mapJobResults(batch []Job, results []JobResult) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	for i, job := range batch {
		resultChan := mb.JobToResultChannel[job]
		resultChan <- results[i]           // Send the result back through the channel
		close(resultChan)                  // Close the result channel
		delete(mb.JobToResultChannel, job) // Clean up the job from the map
	}
}

// failBatch sends error to all jobs in the batch if processing fails after retries
func (mb *MicroBatcher) failBatch(batch []Job, err error) {
	mb.mutex.Lock()
	for _, job := range batch {
		resultChan := mb.JobToResultChannel[job]
		resultChan <- err
		close(resultChan)
		delete(mb.JobToResultChannel, job)
	}
	mb.mutex.Unlock()
}

// processJobs listens for batches of jobs to process.
func (mb *MicroBatcher) processJobs() {
	defer mb.jobsWG.Done() // Signal that job result processing is done when this exits

	for {
		batch, ok := <-mb.batchChan
		if !ok {
			return
		}
		mb.processJob(batch, 1)
	}
}

// Shutdown gracefully stops the MicroBatcher, ensuring all jobs and batches are processed before exiting.
func (mb *MicroBatcher) Shutdown() bool {
	close(mb.addChan)
	mb.additionsWG.Wait()

	close(mb.notifyChan)
	mb.batchesWG.Wait()

	close(mb.batchChan)
	mb.jobsWG.Wait()

	return true
}
