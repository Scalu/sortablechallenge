package sortablechallengeutils

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
)

func runTest(t *testing.T, values []int) {
	var insertSearchSemaphore sync.Mutex
	errorChannel := make(chan string, 1)
	binaryTree := &BinaryTreeParallel{}
	var uniqueValueCount, rebalanceCount, goroutineCount int32
	for valueIndex, value := range values {
		value := value
		valueIndex := valueIndex
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex, len(values))
		insertSearchSemaphore.Lock()
		atomic.AddInt32(&goroutineCount, 1)
		go func() {
			defer func() {
				if x := recover(); x != nil {
					panic(fmt.Sprintf("%s run time panic: %v", logID, x))
				}
			}()
			result := BTPInsert(binaryTree, nil, func(untypedA interface{}, untypedB interface{}) int {
				currentValue := untypedA.(int)
				value := untypedB.(int)
				if value < currentValue {
					return -1
				}
				if value > currentValue {
					return 1
				}
				return 0
			}, value, &insertSearchSemaphore, func() {
				atomic.AddInt32(&rebalanceCount, 1)
			}, &logID)
			if result.(int) != value {
				errorChannel <- fmt.Sprintf("%s Returned value %d does not match value to insert %d", logID, result.(int), value)
			}
			atomic.AddInt32(&goroutineCount, -1)
		}()
	}
	for goroutineCount > 0 { // wait for inserts to finish or an error to occur
		select {
		case errorString := <-errorChannel:
			panic(errorString)
		default:
		}
	}
	t.Logf("Testing with %d total values, %d unique values", len(values), uniqueValueCount)
	// ensure that search works well
	for valueIndex, value := range values {
		insertSearchSemaphore.Lock()
		value := value
		valueIndex := valueIndex
		atomic.AddInt32(&goroutineCount, 1)
		logID := fmt.Sprintf("%d (%d/%d)", value, valueIndex, len(values))
		go func() {
			defer func() {
				if x := recover(); x != nil {
					panic(fmt.Sprintf("%s run time panic: %v", logID, x))
				}
			}()
			result := BTPSearch(binaryTree, func(untypedA interface{}, untypedB interface{}) int {
				currentValue := untypedA.(int)
				value := untypedB.(int)
				if value < currentValue {
					return -1
				}
				if value > currentValue {
					return 1
				}
				return 0
			}, value, &insertSearchSemaphore, &logID)
			if result.(int) != value {
				errorChannel <- fmt.Sprintf("Returned value %d does not match value to find %d", result.(int), value)
			}
			atomic.AddInt32(&goroutineCount, -1)
		}()
	}
	for goroutineCount > 0 { // wait for inserts to finish or an error to occur
		select {
		case errorString := <-errorChannel:
			panic(errorString)
		default:
		}
	}
	// validate the data structure
	iterator := BTPGetFirst(binaryTree)
	previousValue := -1
	for iterator != nil {
		currentValue := BTPGetValue(iterator).(int)
		if currentValue <= previousValue {
			t.Errorf("values out of order: %d then %d", previousValue, currentValue)
			return
		}
		fmt.Printf("prev %d next %d\n", previousValue, currentValue)
		previousValue = currentValue
		if iterator.weight < -1 || iterator.weight > 1 {
			t.Errorf("found value %d with unbalanced weight %d", currentValue, iterator.weight)
			return
		}
		if iterator.possibleWtAdj[0] != 0 || iterator.possibleWtAdj[1] != 0 {
			t.Errorf("found value %d with %d possible inserts/deletes remaining", currentValue, iterator.possibleWtAdj[0]+iterator.possibleWtAdj[1])
			return
		}
		iterator = BTPGetNext(iterator)
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
	for len(values) < numberOfValues {
		randomValue := rand.Intn(maxValue)
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
	//runRandomTest(t, 400, 200)
}
