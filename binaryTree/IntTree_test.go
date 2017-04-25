package binaryTree

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func runTest(t *testing.T, values []int) {
	var insertSearchSemaphore sync.Mutex
	errorChannel := make(chan string, 1)
	defer close(errorChannel)
	jobQueue := make(chan func(), 4+len(values))
	defer close(jobQueue)
	var uniqueValueCount, rebalanceCount, jobCount int32
	binaryTree := &IntTree{
		StoreExtraData: false,
		OnRebalance:    func() { atomic.AddInt32(&rebalanceCount, 1) },
		LaunchNewProcess: func(newProcess func()) {
			atomic.AddInt32(&jobCount, 1)
			jobQueue <- newProcess
		},
		LogFunction: func(getLogLine func() string) bool {
			//if getLogLine != nil {
			//	fmt.Println(getLogLine())
			//}
			//return true
			return false
		}}
	// start the worker processes
	workerProcess := func() {
		var processToRun func()
		okay := true
		for okay {
			processToRun, okay = <-jobQueue
			if okay {
				processToRun()
				atomic.AddInt32(&jobCount, -1)
			}
		}
	}
	for numberOfWorkers := runtime.GOMAXPROCS(0); numberOfWorkers > 0; numberOfWorkers-- {
		go workerProcess()
	}
	// insert values
	for valueIndex, value := range values {
		value := value
		valueIndex := valueIndex
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex+1, len(values))
		logFunction := btpSetLogFunction(binaryTree.LogFunction, logID)
		insertSearchSemaphore.Lock()
		atomic.AddInt32(&jobCount, 1)
		jobQueue <- func() {
			defer func() {
				if x := recover(); x != nil {
					logFunction(func() string { return fmt.Sprintf("run time panic: %v", x) })
					panic(x)
				}
			}()
			binaryTree.InsertValue(value, nil, func(matchFound bool, extraData interface{}) {
				if !matchFound {
					atomic.AddInt32(&uniqueValueCount, 1)
				}
			}, func() { insertSearchSemaphore.Unlock() })
		}
	}
	// ensure that search works well
	for valueIndex, value := range values {
		value := value
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex+1, len(values))
		logFunction := btpSetLogFunction(binaryTree.LogFunction, logID)
		insertSearchSemaphore.Lock()
		atomic.AddInt32(&jobCount, 1)
		jobQueue <- func() {
			defer func() {
				if x := recover(); x != nil {
					logFunction(func() string { return fmt.Sprintf("run time panic: %v", x) })
					panic(x)
				}
			}()
			binaryTree.SearchForValue(value, func(matchFound bool, extraData interface{}) {
				if !matchFound {
					panic(fmt.Sprint("Did not find a match for value ", value))
				}
			}, func() { insertSearchSemaphore.Unlock() })
		}
	}
	// wait for process count to reach zero, or an error to occur
	var firstError string
	for jobCount > 0 {
		select {
		case errorString := <-errorChannel:
			if len(firstError) == 0 {
				firstError = errorString
			}
			fmt.Println("ERROR:", errorString)
		default:
		}
	}
	if len(firstError) > 0 {
		t.Error(firstError)
		return
	}
	// log unique values
	t.Logf("Testing with %d total values, %d unique values", len(values), uniqueValueCount)
	// validate the data structure
	iterator := btpGetFirst(binaryTree.getRootNode())
	var previousValue int
	previousValue = -1
	for iterator != nil {
		currentValue := binaryTree.ints[iterator.valueIndex]
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
			t.Errorf("mismatched left branch boundaries %d should equal %d for node value %d", iterator.branchBoundaries[0], iterator.leftright[0].branchBoundaries[0], binaryTree.ints[iterator.valueIndex])
			return
		}
		if iterator.leftright[0].branchBoundaries[0] == -1 && iterator.branchBoundaries[0] != iterator.valueIndex {
			t.Errorf("invalid left branch boundary %d should equal node value %d", iterator.branchBoundaries[0], binaryTree.ints[iterator.valueIndex])
			return
		}
		if iterator.leftright[1].branchBoundaries[1] != -1 && iterator.branchBoundaries[1] != iterator.leftright[1].branchBoundaries[1] {
			t.Errorf("mismatched right branch boundaries %d should equal %d for node value %d", iterator.branchBoundaries[1], iterator.leftright[1].branchBoundaries[1], binaryTree.ints[iterator.valueIndex])
			return
		}
		if iterator.leftright[1].branchBoundaries[1] == -1 && iterator.branchBoundaries[1] != iterator.valueIndex {
			t.Errorf("invalid right branch boundary %d should equal node value %d", iterator.branchBoundaries[1], binaryTree.ints[iterator.valueIndex])
			return
		}
		iterator = btpGetNext(iterator)
	}
}

// The Binary Tree is a complex piece of code
// We will check the correctness of the tree's structure after indexing values.

func TestSortSingleValue(t *testing.T) {
	runTest(t, []int{1})
}

func TestSortBalancedValues(t *testing.T) {
	runTest(t, []int{2, 1, 3})
}

func TestSortUnbalancedValuesAscending(t *testing.T) {
	runTest(t, []int{1, 2, 3})
}

func TestSortUnbalancedValuesDescending(t *testing.T) {
	runTest(t, []int{3, 2, 1})
}

func TestSortUnbalancedValuesMostlyAscending(t *testing.T) {
	runTest(t, []int{2, 1, 3, 4, 5})
}

func TestSortUnbalancedValuesMostlyDescending(t *testing.T) {
	runTest(t, []int{4, 5, 3, 2, 1})
}

func TestSortUnbalancedValuesComplex(t *testing.T) {
	runTest(t, []int{6, 2, 9, 4, 7, 1, 8, 3, 5})
}

func runRandomTest(t *testing.T, numberOfValues int, maxValue int) {
	// initialize the testData
	values := []int{}
	// seed the tree with random data for 100 ms
	var randomValue int
	for len(values) < numberOfValues {
		randomValue = int(rand.Intn(maxValue))
		values = append(values, randomValue)
	}
	runTest(t, values)
}

func TestSortABitOfRandomData(t *testing.T) {
	runRandomTest(t, 10, 8)
}

func TestSortSomeRandomData(t *testing.T) {
	runRandomTest(t, 20, 12)
}

func TestSortMoreRandomData(t *testing.T) {
	runRandomTest(t, 40, 30)
}

func TestSortLotsOfRandomData(t *testing.T) {
	runRandomTest(t, 400, 200)
}

func TestSort40kOfRandomData(t *testing.T) {
	runRandomTest(t, 40000, 20000)
}

/*
New profile.
Looks like the logger, even when turned off, is generating a lot of mallocs for functions.
And gc spends a lot of time cleaning up these functions.
Other functions being created are also causing problems.

profile commands run and output:
go test -cpuprofile=cprof github.com\Scalu\sortablechallenge\sortablechallengeutils
go tool pprof --text sortablechallengeutils.test.exe cprof
go tool pprof --gif binaryTree.test.exe cprof > graph.gif

64.72s of 69.38s total (93.28%)
Dropped 165 nodes (cum <= 0.35s)
      flat  flat%   sum%        cum   cum%
    11.09s 15.98% 15.98%     16.58s 23.90%  runtime.mallocgc
     9.89s 14.25% 30.24%     13.19s 19.01%  runtime.scanobject
     3.46s  4.99% 35.23%      3.46s  4.99%  runtime.heapBitsSetType
     2.88s  4.15% 39.38%      2.88s  4.15%  runtime.osyield
     2.46s  3.55% 42.92%      2.49s  3.59%  runtime.greyobject
     2.46s  3.55% 46.47%      2.46s  3.55%  runtime.heapBitsForObject
     2.34s  3.37% 49.84%      2.34s  3.37%  runtime.procyield
     2.23s  3.21% 53.06%      4.36s  6.28%  sync.(*Mutex).Lock
     2.08s  3.00% 56.05%      9.39s 13.53%  github.com/Scalu/sortablechallenge/b
inaryTree.(*btpMutex).lock
     1.49s  2.15% 58.20%      1.59s  2.29%  sync.(*Mutex).Unlock
     1.37s  1.97% 60.18%      1.37s  1.97%  runtime.memclrNoHeapPointers
     1.36s  1.96% 62.14%     18.80s 27.10%  github.com/Scalu/sortablechallenge/b
inaryTree.btpDelete
     1.24s  1.79% 63.92%     23.15s 33.37%  runtime.systemstack
     1.18s  1.70% 65.62%      7.77s 11.20%  github.com/Scalu/sortablechallenge/b
inaryTree.(*btpMutex).unlock
     1.18s  1.70% 67.32%      1.26s  1.82%  runtime.writebarrierptr_prewrite1
     0.94s  1.35% 68.68%      2.62s  3.78%  runtime.shade
     0.91s  1.31% 69.99%     16.61s 23.94%  runtime.newobject
     0.90s  1.30% 71.29%      3.52s  5.07%  runtime.gcmarkwb_m
     0.83s  1.20% 72.48%      6.28s  9.05%  github.com/Scalu/sortablechallenge/b
inaryTree.btpInsert.func3
     0.72s  1.04% 73.52%      0.72s  1.04%  runtime.memmove
     0.70s  1.01% 74.53%      0.71s  1.02%  runtime.stdcall2
     0.68s  0.98% 75.51%      1.55s  2.23%  runtime.deferreturn
     0.59s  0.85% 76.36%      2.11s  3.04%  github.com/Scalu/sortablechallenge/b
inaryTree.(*intOperationManager).CompareValueTo
     0.59s  0.85% 77.21%      0.68s  0.98%  runtime.newdefer
     0.57s  0.82% 78.03%      3.76s  5.42%  github.com/Scalu/sortablechallenge/b
inaryTree.btpDelete.func7
     0.53s  0.76% 78.80%      4.05s  5.84%  runtime.writebarrierptr_prewrite1.fu
nc1
     0.43s  0.62% 79.42%      1.67s  2.41%  runtime.lock
     0.38s  0.55% 79.97%      0.67s  0.97%  runtime.freedefer
     0.36s  0.52% 80.48%     38.88s 56.04%  github.com/Scalu/sortablechallenge/b
inaryTree.btpRebalance
     0.35s   0.5% 80.99%      1.67s  2.41%  fmt.(*pp).printArg
     0.35s   0.5% 81.49%      1.19s  1.72%  github.com/Scalu/sortablechallenge/b
inaryTree.btpGetBranchBoundary
     0.35s   0.5% 82.00%      0.35s   0.5%  runtime.markBits.setMarked
     0.34s  0.49% 82.49%      0.36s  0.52%  runtime.unlock
     0.32s  0.46% 82.95%      1.34s  1.93%  github.com/Scalu/sortablechallenge/b
inaryTree.(*intOperationManager).CloneWithStoredValue
     0.30s  0.43% 83.38%      0.51s  0.74%  sync.(*Pool).pin
     0.28s   0.4% 83.78%      0.54s  0.78%  fmt.(*fmt).fmt_integer
     0.26s  0.37% 84.16%      0.41s  0.59%  runtime.bulkBarrierPreWrite
     0.24s  0.35% 84.51%      1.96s  2.83%  fmt.(*pp).doPrint
     0.24s  0.35% 84.85%     15.78s 22.74%  github.com/Scalu/sortablechallenge/b
inaryTree.btpInsert
     0.24s  0.35% 85.20%      1.97s  2.84%  runtime.convT2E
     0.24s  0.35% 85.54%      1.07s  1.54%  runtime.gcmarknewobject
     0.24s  0.35% 85.89%      1.35s  1.95%  runtime.writebarrierptr
     0.21s   0.3% 86.19%         1s  1.44%  runtime.deferproc
     0.20s  0.29% 86.48%      2.72s  3.92%  runtime.(*mcentral).cacheSpan
     0.19s  0.27% 86.75%      0.76s  1.10%  fmt.newPrinter
     0.19s  0.27% 87.03%     12.07s 17.40%  github.com/Scalu/sortablechallenge/b
inaryTree.btpStep
     0.18s  0.26% 87.29%      2.77s  3.99%  github.com/Scalu/sortablechallenge/b
inaryTree.btpRebalanceIfNecessary
     0.18s  0.26% 87.55%      3.21s  4.63%  runtime.findrunnable
     0.18s  0.26% 87.81%      6.31s  9.09%  runtime.gcDrain
     0.17s  0.25% 88.05%      4.80s  6.92%  fmt.Sprint
     0.17s  0.25% 88.30%      7.79s 11.23%  runtime.gcDrainN
     0.16s  0.23% 88.53%      0.38s  0.55%  fmt.(*fmt).padString
     0.16s  0.23% 88.76%      2.15s  3.10%  github.com/Scalu/sortablechallenge/b
inaryTree.btpAdjustWeightAndPossibleWtAdj
     0.16s  0.23% 88.99%      0.58s  0.84%  github.com/Scalu/sortablechallenge/b
inaryTree.btpDelete.func7.2
     0.16s  0.23% 89.22%      3.44s  4.96%  github.com/Scalu/sortablechallenge/b
inaryTree.btpInsert.func3.1
     0.16s  0.23% 89.45%      1.17s  1.69%  runtime.rawstringtmp
     0.16s  0.23% 89.68%      0.39s  0.56%  sync.(*Pool).Put
     0.15s  0.22% 89.90%     41.69s 60.09%  github.com/Scalu/sortablechallenge/b
inaryTree.runTest.func4
     0.15s  0.22% 90.11%      0.87s  1.25%  runtime.sweepone
     0.15s  0.22% 90.33%      0.69s  0.99%  runtime.typedmemmove
     0.14s   0.2% 90.53%      0.55s  0.79%  github.com/Scalu/sortablechallenge/b
inaryTree.(*intOperationManager).LaunchNewProcess
     0.14s   0.2% 90.73%      1.01s  1.46%  runtime.rawstring
     0.14s   0.2% 90.93%      1.40s  2.02%  runtime.slicebytetostring
     0.13s  0.19% 91.12%      0.73s  1.05%  runtime.(*mheap).alloc_m
     0.13s  0.19% 91.31%      0.55s  0.79%  sync.(*Pool).Get
     0.10s  0.14% 91.45%      0.61s  0.88%  fmt.(*pp).fmtString
     0.10s  0.14% 91.60%      0.58s  0.84%  fmt.(*pp).free
     0.10s  0.14% 91.74%      1.95s  2.81%  github.com/Scalu/sortablechallenge/b
inaryTree.btpDelete.func7.1
     0.10s  0.14% 91.89%      0.38s  0.55%  runtime.(*mcentral).freeSpan
     0.09s  0.13% 92.01%      0.76s  1.10%  runtime.(*mspan).sweep
     0.08s  0.12% 92.13%      0.51s  0.74%  fmt.(*fmt).fmt_s
     0.08s  0.12% 92.25%      0.62s  0.89%  fmt.(*pp).fmtInteger
     0.07s   0.1% 92.35%      1.18s  1.70%  github.com/Scalu/sortablechallenge/b
inaryTree.(*intOperationManager).GetStoredValueString
     0.07s   0.1% 92.45%      1.68s  2.42%  github.com/Scalu/sortablechallenge/b
inaryTree.btpAdjustPossibleWtAdj
     0.07s   0.1% 92.55%      0.41s  0.59%  github.com/Scalu/sortablechallenge/b
inaryTree.runTest.func2
     0.07s   0.1% 92.65%      3.55s  5.12%  runtime.schedule
     0.06s 0.086% 92.74%      2.78s  4.01%  runtime.runqgrab
     0.05s 0.072% 92.81%     38.93s 56.11%  github.com/Scalu/sortablechallenge/b
inaryTree.btpRebalanceIfNecessary.func1
     0.04s 0.058% 92.87%      7.90s 11.39%  runtime.gcAssistAlloc1
     0.03s 0.043% 92.91%      0.46s  0.66%  github.com/Scalu/sortablechallenge/b
inaryTree.btpRebalance.func3
     0.03s 0.043% 92.95%      2.75s  3.96%  runtime.(*mcache).refill
     0.03s 0.043% 93.00%      1.87s  2.70%  runtime.(*mcentral).grow
     0.03s 0.043% 93.04%      0.45s  0.65%  runtime.chanrecv
     0.02s 0.029% 93.07%      1.96s  2.83%  github.com/Scalu/sortablechallenge/b
inaryTree.(*intOperationManager).GetValueString
     0.02s 0.029% 93.10%      0.79s  1.14%  github.com/Scalu/sortablechallenge/b
inaryTree.runTest
     0.02s 0.029% 93.12%      1.55s  2.23%  runtime.(*mheap).alloc
     0.02s 0.029% 93.15%      0.75s  1.08%  runtime.(*mheap).alloc.func1
     0.02s 0.029% 93.18%      0.78s  1.12%  runtime.gosweepone.func1
     0.02s 0.029% 93.21%      3.07s  4.42%  runtime.mcall
     0.02s 0.029% 93.24%      2.75s  3.96%  runtime.park_m
     0.01s 0.014% 93.25%      1.96s  2.83%  github.com/Scalu/sortablechallenge/b
inaryTree.btpDelete.func4
     0.01s 0.014% 93.27%      0.39s  0.56%  runtime.chansend
     0.01s 0.014% 93.28%      2.79s  4.02%  runtime.runqsteal
         0     0% 93.28%      2.01s  2.90%  github.com/Scalu/sortablechallenge/b
inaryTree.(*IntTree).InsertValue
         0     0% 93.28%      0.79s  1.14%  github.com/Scalu/sortablechallenge/b
inaryTree.TestSort40kOfRandomData
         0     0% 93.28%      0.79s  1.14%  github.com/Scalu/sortablechallenge/b
inaryTree.runRandomTest
         0     0% 93.28%      2.04s  2.94%  github.com/Scalu/sortablechallenge/b
inaryTree.runTest.func5
         0     0% 93.28%      2.75s  3.96%  runtime.(*mcache).nextFree.func1
         0     0% 93.28%      1.28s  1.84%  runtime._System
         0     0% 93.28%      0.45s  0.65%  runtime.chanrecv2
         0     0% 93.28%      0.39s  0.56%  runtime.chansend1
         0     0% 93.28%      7.90s 11.39%  runtime.gcAssistAlloc.func1
         0     0% 93.28%      6.31s  9.09%  runtime.gcBgMarkWorker.func2
         0     0% 93.28%     42.53s 61.30%  runtime.goexit
         0     0% 93.28%      0.67s  0.97%  runtime.gopreempt_m
         0     0% 93.28%      0.97s  1.40%  runtime.goschedImpl
         0     0% 93.28%      0.68s  0.98%  runtime.morestack
         0     0% 93.28%      0.68s  0.98%  runtime.newstack
         0     0% 93.28%      0.71s  1.02%  runtime.semasleep
         0     0% 93.28%     21.84s 31.48%  runtime.startTheWorldWithSema
         0     0% 93.28%      1.91s  2.75%  sync.runtime_doSpin
         0     0% 93.28%      0.79s  1.14%  testing.tRunner
*/
