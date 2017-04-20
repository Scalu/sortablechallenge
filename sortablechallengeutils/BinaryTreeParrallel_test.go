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
	logFunction := func(getLogLine func() string) bool {
		if getLogLine != nil {
			fmt.Println(getLogLine())
		}
		return true
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
		fmt.Printf("prev %d next %d\n", previousValue, currentValue)
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
