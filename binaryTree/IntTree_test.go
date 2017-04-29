package binaryTree

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func runTest(t *testing.T, values []int, enableLogging bool) {
	var insertSearchSemaphore sync.Mutex
	unlockInsertSearchSemaphore := func() { insertSearchSemaphore.Unlock() }
	errorChannel := make(chan string, 1)
	defer close(errorChannel)
	jobQueue := make(chan func(int, int32), 1)
	workerShutdownSignalChannel := make(chan int)
	defer close(workerShutdownSignalChannel)
	var uniqueValueCount, rebalanceCount int32
	binaryTree := &IntTree{
		StoreExtraData: false,
		OnRebalance:    func() { atomic.AddInt32(&rebalanceCount, 1) },
		LogFunction: func(logLine string) bool {
			return false
		}}
	testStartTime := time.Now()
	if enableLogging {
		binaryTree.LogFunction = func(logLine string) bool {
			if len(logLine) != 0 {
				fmt.Println(time.Since(testStartTime), logLine)
			}
			return true
		}
	}
	// start the worker processes
	var jobNumber int32
	workerProcess := func(workerID int) {
		var processToRun func(int, int32)
		okay := true
		for okay {
			processToRun, okay = <-jobQueue
			if okay {
				jobNumber := atomic.AddInt32(&jobNumber, 1) - 1
				processToRun(workerID, jobNumber)
			}
		}
		workerShutdownSignalChannel <- 1
	}
	numberOfWorkers := runtime.GOMAXPROCS(0)
	for workerID := 0; workerID < numberOfWorkers; workerID++ {
		go workerProcess(workerID)
	}
	// insert values
	for valueIndex, value := range values {
		value := value
		valueIndex := valueIndex
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex+1, len(values))
		logFunction := binaryTree.LogFunction
		if logFunction("") {
			logFunction = btpSetLogFunction(binaryTree.LogFunction, logID)
		}
		insertSearchSemaphore.Lock()
		jobQueue <- func(workerID int, jobID int32) {
			defer func() {
				if x := recover(); x != nil {
					if logFunction("") {
						logFunction(fmt.Sprintf("run time panic: %v", x))
					}
					panic(x)
				}
			}()
			binaryTree.InsertValue(value, nil, func(matchFound bool, extraData interface{}) {
				if !matchFound {
					atomic.AddInt32(&uniqueValueCount, 1)
				}
			}, unlockInsertSearchSemaphore, fmt.Sprint(workerID, ":", jobID))
		}
	}
	// ensure that search works well
	for valueIndex, value := range values {
		value := value
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex+1, len(values))
		logFunction := binaryTree.LogFunction
		if logFunction("") {
			logFunction = btpSetLogFunction(binaryTree.LogFunction, logID)
		}
		insertSearchSemaphore.Lock()
		jobQueue <- func(workerID int, jobID int32) {
			defer func() {
				if x := recover(); x != nil {
					if logFunction("") {
						logFunction(fmt.Sprintf("run time panic: %v", x))
					}
					panic(x)
				}
			}()
			binaryTree.SearchForValue(value, func(matchFound bool, extraData interface{}) {
				if !matchFound {
					panic(fmt.Sprint("Did not find a match for value ", value))
				}
			}, unlockInsertSearchSemaphore, fmt.Sprint(workerID, ":", jobID))
		}
	}
	// wait for process count to reach zero, or an error to occur
	var firstError string
	var workersShutDown int
	close(jobQueue)
	for workersShutDown < numberOfWorkers {
		select {
		case errorString := <-errorChannel:
			if len(firstError) == 0 {
				firstError = errorString
			}
			fmt.Println("ERROR:", errorString)
		case <-workerShutdownSignalChannel:
			workersShutDown++
		}
	}
	if len(firstError) > 0 {
		t.Error(firstError)
		return
	}
	// log unique values
	t.Logf("Testing with %d total values, %d unique values, %d rebalances", len(values), uniqueValueCount, rebalanceCount)
	// validate the data structure
	iterator := btpGetFirst(binaryTree.getRootNode())
	var previousValue int
	previousValue = -1
	for iterator != nil {
		currentValue := binaryTree.ints[iterator.valueIndex>>8][iterator.valueIndex&255]
		if currentValue <= previousValue {
			t.Errorf("values out of order: %d then %d", previousValue, currentValue)
			return
		}
		// fmt.Printf("prev %d next %d\n", previousValue, currentValue)
		previousValue = currentValue
		if iterator.currentWeight < -1 || iterator.currentWeight > 1 {
			t.Errorf("found value %d with unbalanced weight %d", currentValue, iterator.currentWeight)
			return
		}
		if iterator.possibleWtAdjust[0] != 0 || iterator.possibleWtAdjust[1] != 0 {
			t.Errorf("found value %d with %d possible inserts/deletes remaining", currentValue, iterator.possibleWtAdjust[0]+iterator.possibleWtAdjust[1])
			return
		}
		if iterator.leftright[0].branchBoundaries[0] != -1 && iterator.branchBoundaries[0] != iterator.leftright[0].branchBoundaries[0] {
			t.Errorf("mismatched left branch boundary index %d should equal %d for node value %d", iterator.branchBoundaries[0], iterator.leftright[0].branchBoundaries[0], currentValue)
			return
		}
		if iterator.leftright[0].branchBoundaries[0] == -1 && iterator.branchBoundaries[0] != iterator.valueIndex {
			t.Errorf("invalid left branch boundary index %d should equal node value index %d", iterator.branchBoundaries[0], currentValue)
			return
		}
		if iterator.leftright[1].branchBoundaries[1] != -1 && iterator.branchBoundaries[1] != iterator.leftright[1].branchBoundaries[1] {
			t.Errorf("mismatched right branch boundary index %d should equal %d for node value %d", iterator.branchBoundaries[1], iterator.leftright[1].branchBoundaries[1], currentValue)
			return
		}
		if iterator.leftright[1].branchBoundaries[1] == -1 && iterator.branchBoundaries[1] != iterator.valueIndex {
			t.Errorf("invalid right branch boundary index %d should equal node value index %d", iterator.branchBoundaries[1], currentValue)
			return
		}
		iterator = btpGetNext(iterator)
	}
}

// The Binary Tree is a complex piece of code
// We will check the correctness of the tree's structure after indexing values.

func TestSortSingleValue(t *testing.T) {
	runTest(t, []int{1}, false)
}

func TestSortBalancedValues(t *testing.T) {
	runTest(t, []int{2, 1, 3}, false)
}

func TestSortUnbalancedValuesAscending(t *testing.T) {
	runTest(t, []int{1, 2, 3}, false)
}

func TestSortUnbalancedValuesDescending(t *testing.T) {
	runTest(t, []int{3, 2, 1}, false)
}

func TestSortUnbalancedValuesMostlyAscending(t *testing.T) {
	runTest(t, []int{2, 1, 3, 4, 5}, false)
}

func TestSortUnbalancedValuesMostlyDescending(t *testing.T) {
	runTest(t, []int{4, 5, 3, 2, 1}, false)
}

func TestSortUnbalancedValuesComplex(t *testing.T) {
	runTest(t, []int{6, 2, 9, 4, 7, 1, 8, 3, 5}, false)
}

func runRandomTest(t *testing.T, numberOfValues int, maxValue int, enableLogging bool) {
	// initialize the testData
	values := []int{}
	// seed the tree with random data for 100 ms
	var randomValue int
	for len(values) < numberOfValues {
		randomValue = int(rand.Intn(maxValue))
		values = append(values, randomValue)
	}
	runTest(t, values, enableLogging)
}

func TestSortABitOfRandomData(t *testing.T) {
	runRandomTest(t, 10, 8, false)
}

func TestSortSomeRandomData(t *testing.T) {
	runRandomTest(t, 20, 12, false)
}

func TestSortMoreRandomData(t *testing.T) {
	runRandomTest(t, 40, 30, false)
}

func TestSortLotsOfRandomData(t *testing.T) {
	runRandomTest(t, 400, 200, true)
}

func TestSort4kOfRandomData(t *testing.T) {
	runRandomTest(t, 4000, 2000, false)
}

func TestSort40kOfRandomData(t *testing.T) {
	runRandomTest(t, 40000, 20000, false)
}

/*
Fixed the logger and got a lot of performance back. Going to keep working on performance issues.

profile commands run and output:
go test -cpuprofile=cprof .
go tool pprof --text binaryTree.test.exe cprof > cprof.txt
go tool pprof --gif binaryTree.test.exe cprof > graph.gif
*/
