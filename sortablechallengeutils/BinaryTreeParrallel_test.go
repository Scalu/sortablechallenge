package sortablechallengeutils

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
)

type binaryTreeIntValue int

func (value binaryTreeIntValue) CompareTo(currentRawValue BinaryTreeValue) int {
	currentValue := currentRawValue.(binaryTreeIntValue)
	if value < currentValue {
		return -1
	}
	if value > currentValue {
		return 1
	}
	return 0
}

func (value binaryTreeIntValue) ToString() string {
	return fmt.Sprint(value)
}

func runTest(t *testing.T, values []binaryTreeIntValue) {
	var insertSearchSemaphore sync.Mutex
	errorChannel := make(chan string, 1)
	defer close(errorChannel)
	binaryTree := &BinaryTreeParallel{}
	var uniqueValueCount, rebalanceCount int32
	/* logFunction := func(getLogLine func() string) bool {
		if getLogLine != nil {
			fmt.Println(getLogLine())
		}
		return true
	} */
	logFunction := func(func() string) bool {
		return false
	}
	// start the process counter
	processCountChannel := make(chan int, 1)
	defer close(processCountChannel)
	processCounterChannel := make(chan int)
	defer close(processCounterChannel)
	go func() {
		processCount := 0
		returnZeroCount := false
		okay := true
		var countAdjustment int
		for okay {
			select {
			case countAdjustment, okay = <-processCounterChannel:
				if countAdjustment == 0 && okay {
					returnZeroCount = true
				} else {
					processCount += countAdjustment
					logFunction(func() string { return fmt.Sprintf("Process count: %d", processCount) })
				}
				if processCount == 0 && returnZeroCount {
					processCountChannel <- processCount
					returnZeroCount = false
				}
			}
		}
	}()
	// insert values
	for valueIndex, value := range values {
		processCounterChannel <- 1
		value := value
		valueIndex := valueIndex
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex+1, len(values))
		logFunction := btpSetLogFunction(logFunction, logID)
		insertSearchSemaphore.Lock()
		go func() {
			defer func() {
				if x := recover(); x != nil {
					logFunction(func() string { return fmt.Sprintf("run time panic: %v", x) })
					panic(x)
				}
				processCounterChannel <- -1
			}()
			result, matchFound := BTPInsert(binaryTree, value, &insertSearchSemaphore, func() {
				atomic.AddInt32(&rebalanceCount, 1)
			}, logFunction, processCounterChannel)
			if !matchFound {
				atomic.AddInt32(&uniqueValueCount, 1)
			}
			if result == nil {
				logFunction(func() string { return "Invalid nil result" })
				panic("Invalid nil result")
			}
			if value.CompareTo(result) != 0 {
				errorChannel <- fmt.Sprintf("%s Returned value %s does not match value to insert %s", logID, result.ToString(), value.ToString())
			}
		}()
	}
	// ensure that search works well
	for valueIndex, value := range values {
		processCounterChannel <- 1
		value := value
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex+1, len(values))
		logFunction := btpSetLogFunction(logFunction, logID)
		insertSearchSemaphore.Lock()
		go func() {
			defer func() {
				if x := recover(); x != nil {
					logFunction(func() string { return fmt.Sprintf("run time panic: %v", x) })
					panic(x)
				}
				processCounterChannel <- -1
			}()
			result := BTPSearch(binaryTree, value, &insertSearchSemaphore, logFunction)
			if result == nil {
				logFunction(func() string { return "Invalid nil result" })
				panic("Invalid nil result")
			}
			if value.CompareTo(result) != 0 {
				errorChannel <- fmt.Sprintf("%s Returned value %s does not match value to find %s", logID, result.ToString(), value.ToString())
			}
		}()
	}
	processCounterChannel <- 0 // signal process counter to return when count is zero
	// wait for process count to reach zero, or an error to occur
	var firstError string
	var processCountReceived bool
	for !processCountReceived {
		select {
		case <-processCountChannel:
			processCountReceived = true
		case errorString := <-errorChannel:
			if len(firstError) == 0 {
				firstError = errorString
			}
			fmt.Println("ERROR:", errorString)
		}
	}
	if len(firstError) > 0 {
		t.Error(firstError)
		return
	}
	// log unique values
	t.Logf("Testing with %d total values, %d unique values", len(values), uniqueValueCount)
	// validate the data structure
	iterator := BTPGetFirst(binaryTree)
	var previousValue binaryTreeIntValue
	previousValue = -1
	for iterator != nil {
		currentValue := iterator.value.(binaryTreeIntValue)
		if currentValue.CompareTo(previousValue) != 1 {
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
		if iterator.leftright[0].branchBoundaries[0] != nil && iterator.branchBoundaries[0] != iterator.leftright[0].branchBoundaries[0] {
			t.Errorf("mismatched left branch boundaries %d should equal %d for node value %d", iterator.branchBoundaries[0], iterator.leftright[0].branchBoundaries[0], iterator.value)
			return
		}
		if iterator.leftright[0].branchBoundaries[0] == nil && iterator.branchBoundaries[0] != iterator.value {
			t.Errorf("invalid left branch boundary %d should equal node value %d", iterator.branchBoundaries[0], iterator.value)
			return
		}
		if iterator.leftright[1].branchBoundaries[1] != nil && iterator.branchBoundaries[1] != iterator.leftright[1].branchBoundaries[1] {
			t.Errorf("mismatched right branch boundaries %d should equal %d for node value %d", iterator.branchBoundaries[1], iterator.leftright[1].branchBoundaries[1], iterator.value)
			return
		}
		if iterator.leftright[1].branchBoundaries[1] == nil && iterator.branchBoundaries[1] != iterator.value {
			t.Errorf("invalid right branch boundary %d should equal node value %d", iterator.branchBoundaries[1], iterator.value)
			return
		}
		iterator = BTPGetNext(iterator)
	}
}

// The Binary Tree is a complex piece of code
// We will check the correctness of the tree's structure after indexing values.

func TestSortSingleValue(t *testing.T) {
	runTest(t, []binaryTreeIntValue{1})
}

func TestSortBalancedValues(t *testing.T) {
	runTest(t, []binaryTreeIntValue{2, 1, 3})
}

func TestSortUnbalancedValuesAscending(t *testing.T) {
	runTest(t, []binaryTreeIntValue{1, 2, 3})
}

func TestSortUnbalancedValuesDescending(t *testing.T) {
	runTest(t, []binaryTreeIntValue{3, 2, 1})
}

func TestSortUnbalancedValuesMostlyAscending(t *testing.T) {
	runTest(t, []binaryTreeIntValue{2, 1, 3, 4, 5})
}

func TestSortUnbalancedValuesMostlyDescending(t *testing.T) {
	runTest(t, []binaryTreeIntValue{4, 5, 3, 2, 1})
}

func TestSortUnbalancedValuesComplex(t *testing.T) {
	runTest(t, []binaryTreeIntValue{6, 2, 9, 4, 7, 1, 8, 3, 5})
}

func runRandomTest(t *testing.T, numberOfValues int, maxValue int) {
	// initialize the testData
	values := []binaryTreeIntValue{}
	// seed the tree with random data for 100 ms
	var randomValue binaryTreeIntValue
	for len(values) < numberOfValues {
		randomValue = binaryTreeIntValue(rand.Intn(maxValue))
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
profiled the run of the current implementation, and that last test with 40k records takes 25 seconds, where my single-threaded test takes 0.1 seconds with about 30k records
I am guessing that the different data types being stored are to blame, causing excessive GC perhaps. Also, I should limit the number of goroutines (right now hundreds of them are running simultaneously)
I will look into this another day.

profile commands run and output:
go test -cpuprofile=cprof github.com\Scalu\sortablechallenge\sortablechallengeutils
go tool pprof --text sortablechallengeutils.test.exe cprof

76.53s of 84.64s total (90.42%)
Dropped 184 nodes (cum <= 0.42s)
      flat  flat%   sum%        cum   cum%
     9.48s 11.20% 11.20%      9.51s 11.24%  runtime.stdcall2
     8.06s  9.52% 20.72%     11.69s 13.81%  runtime.mallocgc
     5.36s  6.33% 27.06%      5.39s  6.37%  runtime.osyield
     5.09s  6.01% 33.07%      8.32s  9.83%  runtime.scanobject
     4.79s  5.66% 38.73%      4.79s  5.66%  runtime.procyield
     2.47s  2.92% 41.65%      2.47s  2.92%  runtime.memmove
     2.33s  2.75% 44.40%      2.33s  2.75%  runtime.greyobject
     2.18s  2.58% 46.98%      2.18s  2.58%  runtime.heapBitsSetType
     2.03s  2.40% 49.37%      3.02s  3.57%  runtime.deferreturn
     1.63s  1.93% 51.30%     18.43s 21.77%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete
     1.62s  1.91% 53.21%      1.62s  1.91%  runtime.heapBitsForObject
     1.58s  1.87% 55.08%      1.65s  1.95%  runtime.gopark
     1.39s  1.64% 56.72%      2.19s  2.59%  sync.(*Mutex).Lock
     1.35s  1.59% 58.32%      1.46s  1.72%  runtime.casgstatus
     1.23s  1.45% 59.77%      5.19s  6.13%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.(*btpMutex).Lock
     1.23s  1.45% 61.22%     20.40s 24.10%  runtime.lock
     1.18s  1.39% 62.62%     21.06s 24.88%  runtime.chansend
     0.93s  1.10% 63.72%      4.76s  5.62%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.(*btpMutex).Unlock
     0.92s  1.09% 64.80%      1.13s  1.34%  sync.(*Mutex).Unlock
     0.90s  1.06% 65.87%      3.56s  4.21%  runtime.schedule
     0.89s  1.05% 66.92%     11.97s 14.14%  runtime.newobject
     0.88s  1.04% 67.96%      0.88s  1.04%  runtime.memclrNoHeapPointers
     0.86s  1.02% 68.97%      0.95s  1.12%  runtime.unlock
     0.78s  0.92% 69.90%      0.78s  0.92%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.(*binaryTreeIntValue).CompareTo
     0.76s   0.9% 70.79%      0.80s  0.95%  runtime.newdefer
     0.73s  0.86% 71.66%      1.85s  2.19%  runtime.deferproc
     0.62s  0.73% 72.39%      0.73s  0.86%  runtime.freedefer
     0.60s  0.71% 73.10%      4.23s  5.00%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.(*binaryTreeIntValue).ToString
     0.57s  0.67% 73.77%      2.22s  2.62%  runtime.goparkunlock
     0.49s  0.58% 74.35%      0.49s  0.58%  runtime.getitab
     0.48s  0.57% 74.92%      0.96s  1.13%  runtime.runqput
     0.48s  0.57% 75.48%     17.33s 20.47%  runtime.systemstack
     0.47s  0.56% 76.04%      1.19s  1.41%  runtime.pcvalue
     0.44s  0.52% 76.56%      0.60s  0.71%  fmt.(*fmt).fmt_integer
     0.43s  0.51% 77.07%      3.15s  3.72%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPInsert.func7
     0.42s   0.5% 77.56%     11.62s 13.73%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalance
     0.42s   0.5% 78.06%      2.18s  2.58%  runtime.gentraceback
     0.41s  0.48% 78.54%      0.93s  1.10%  runtime.newproc1
     0.38s  0.45% 78.99%      6.67s  7.88%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.runTest.func2
     0.36s  0.43% 79.42%      2.19s  2.59%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete.func9
     0.34s   0.4% 79.82%      2.48s  2.93%  fmt.(*pp).printArg
     0.34s   0.4% 80.22%      1.15s  1.36%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpGetBranchBoundary
     0.34s   0.4% 80.62%      1.08s  1.28%  runtime.gcmarkwb_m
     0.33s  0.39% 81.01%      0.45s  0.53%  runtime.globrunqget
     0.31s  0.37% 81.38%     13.55s 16.01%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPInsert
     0.31s  0.37% 81.75%      0.86s  1.02%  runtime.rawstring
     0.30s  0.35% 82.10%      5.27s  6.23%  runtime.mcall
     0.30s  0.35% 82.46%      2.62s  3.10%  runtime.recv
     0.27s  0.32% 82.77%      0.62s  0.73%  runtime.writebarrierptr_prewrite1
     0.26s  0.31% 83.08%      0.74s  0.87%  runtime.shade
     0.24s  0.28% 83.36%     16.23s 19.18%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalance.func5
     0.24s  0.28% 83.65%      0.57s  0.67%  runtime.step
     0.22s  0.26% 83.91%      7.39s  8.73%  runtime.gcDrainN
     0.21s  0.25% 84.16%      0.94s  1.11%  runtime.(*mcentral).cacheSpan
     0.21s  0.25% 84.40%      0.70s  0.83%  runtime.assertE2I2
     0.20s  0.24% 84.64%      0.92s  1.09%  fmt.(*pp).handleMethods
     0.20s  0.24% 84.88%      6.55s  7.74%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalanceIfNecessary
     0.20s  0.24% 85.11%      1.49s  1.76%  runtime.ready
     0.20s  0.24% 85.35%      0.76s   0.9%  runtime.sweepone
     0.19s  0.22% 85.57%      0.91s  1.08%  runtime.execute
     0.18s  0.21% 85.79%      0.64s  0.76%  runtime.semacquire
     0.18s  0.21% 86.00%      1.26s  1.49%  runtime.writebarrierptr_prewrite1.fu
nc1
     0.17s   0.2% 86.20%      2.68s  3.17%  fmt.(*pp).doPrint
     0.16s  0.19% 86.39%     18.66s 22.05%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalance.func6
     0.16s  0.19% 86.58%      1.35s  1.59%  runtime.convT2E
     0.15s  0.18% 86.76%      4.73s  5.59%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPInsert.func7.1
     0.14s  0.17% 86.92%      4.92s  5.81%  fmt.Sprint
     0.14s  0.17% 87.09%      0.55s  0.65%  fmt.newPrinter
     0.14s  0.17% 87.25%      6.42s  7.59%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpStep
     0.14s  0.17% 87.42%      1.53s  1.81%  runtime.goexit0
     0.13s  0.15% 87.57%      2.13s  2.52%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalanceIfNecessary.func1.1
     0.13s  0.15% 87.72%      4.96s  5.86%  runtime.chanrecv
     0.13s  0.15% 87.88%      1.87s  2.21%  runtime.recvDirect
     0.12s  0.14% 88.02%      3.31s  3.91%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete.func9.1
     0.12s  0.14% 88.16%     21.19s 25.04%  runtime.chansend1
     0.12s  0.14% 88.30%      3.26s  3.85%  runtime.park_m
     0.12s  0.14% 88.45%      1.18s  1.39%  runtime.slicebytetostring
     0.11s  0.13% 88.58%      1.40s  1.65%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpAdjustWeightAndPossibleWtAdj
     0.10s  0.12% 88.69%      0.79s  0.93%  fmt.(*pp).printValue
     0.10s  0.12% 88.81%         5s  5.91%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete.func12
     0.10s  0.12% 88.93%      1.22s  1.44%  runtime.scanframeworker
     0.09s  0.11% 89.04%      0.48s  0.57%  runtime.(*mspan).sweep
     0.09s  0.11% 89.14%      1.57s  1.85%  runtime.goready.func1
     0.09s  0.11% 89.25%      0.95s  1.12%  runtime.rawstringtmp
     0.07s 0.083% 89.33%      1.50s  1.77%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpAdjustPossibleWtAdj
     0.07s 0.083% 89.41%     13.96s 16.49%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalanceIfNecessary.func1
     0.07s 0.083% 89.50%      0.64s  0.76%  runtime.writebarrierptr
     0.06s 0.071% 89.57%      3.63s  4.29%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.binaryTreeIntValue.ToString
     0.06s 0.071% 89.64%      5.02s  5.93%  runtime.chanrecv2
     0.05s 0.059% 89.70%      0.65s  0.77%  fmt.(*pp).fmtInteger
     0.05s 0.059% 89.76%      3.37s  3.98%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete.func5
     0.05s 0.059% 89.82%      0.72s  0.85%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete.func8
     0.05s 0.059% 89.87%      3.95s  4.67%  runtime.gcDrain
     0.04s 0.047% 89.92%      3.05s  3.60%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalance.func5.1
     0.04s 0.047% 89.97%      2.66s  3.14%  runtime.markroot
     0.04s 0.047% 90.02%      0.97s  1.15%  runtime.newproc.func1
     0.03s 0.035% 90.05%      2.04s  2.41%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPDelete.func12.1
     0.03s 0.035% 90.09%      4.79s  5.66%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.BTPInsert.func5
     0.03s 0.035% 90.12%         1s  1.18%  runtime.(*mcache).nextFree.func1
     0.03s 0.035% 90.16%      0.97s  1.15%  runtime.(*mcache).refill
     0.03s 0.035% 90.19%      0.43s  0.51%  runtime.chanrecv.func1
     0.03s 0.035% 90.23%      0.59s   0.7%  runtime.funcspdelta
     0.03s 0.035% 90.26%      2.39s  2.82%  runtime.scanstack
     0.03s 0.035% 90.30%      0.67s  0.79%  sync.runtime_SemacquireMutex
     0.02s 0.024% 90.32%      2.79s  3.30%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalance.func6.1
     0.02s 0.024% 90.35%      1.06s  1.25%  runtime.findrunnable
     0.02s 0.024% 90.37%      7.50s  8.86%  runtime.gcAssistAlloc1
     0.01s 0.012% 90.38%      0.78s  0.92%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.btpRebalance.func3
     0.01s 0.012% 90.39%      7.51s  8.87%  runtime.gcAssistAlloc.func1
     0.01s 0.012% 90.41%      1.23s  1.45%  runtime.scanstack.func1
     0.01s 0.012% 90.42%      9.52s 11.25%  runtime.semasleep
         0     0% 90.42%      0.84s  0.99%  github.com/Scalu/sortablechallenge/s
ortablechallengeutils.runTest.func3
         0     0% 90.42%      1.20s  1.42%  runtime._System
         0     0% 90.42%      3.95s  4.67%  runtime.gcBgMarkWorker.func2
         0     0% 90.42%     61.83s 73.05%  runtime.goexit
         0     0% 90.42%      0.61s  0.72%  runtime.gosweepone.func1
         0     0% 90.42%      2.45s  2.89%  runtime.markroot.func1
         0     0% 90.42%      0.63s  0.74%  runtime.pcdatavalue
         0     0% 90.42%      2.45s  2.89%  runtime.scang
         0     0% 90.42%     16.53s 19.53%  runtime.startTheWorldWithSema
*/
