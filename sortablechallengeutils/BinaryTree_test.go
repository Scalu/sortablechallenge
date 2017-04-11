package sortablechallengeutils

import (
	"math/rand"
	"testing"
	"time"
)

// The Binary Tree is a complex piece of code, and checking individual operations will be time consuming.
// For now, we will check the correctness of the tree's structure after indexing values.

type testDataStruct struct {
	negativeValue int
	values        []int
	binaryTree    BinaryTree
}

func (tds *testDataStruct) BinaryTreeCompare(valueA, valueB int) int {
	if valueA == -1 {
		valueA = tds.negativeValue
	}
	if valueB == -1 {
		valueB = tds.negativeValue
	}
	if valueA < valueB {
		return 1
	}
	if valueA > valueB {
		return -1
	}
	return 0
}

func (tds *testDataStruct) GetInsertValue() int {
	return tds.negativeValue
}

// TestSortRandomData Inserts a bunch of random data into the binary tree for now
// checks the tree for correctness and then searches for each value
func TestSortRandomData(t *testing.T) {
	// initialize the testData
	testData := testDataStruct{}
	testData.binaryTree.Comparer = &testData
	// seed the tree with random data for 100 ms
	startTime := time.Now()
	uniqueValueCount := 0
	for time.Since(startTime) < time.Millisecond*100 {
		testData.negativeValue = rand.Intn(20000)
		testData.values = append(testData.values, testData.negativeValue)
		newValue, alreadyExists := testData.binaryTree.Insert(-1, false)
		if newValue != testData.negativeValue {
			t.Errorf("did not return correct value %d when inserting a value, returned %d", testData.negativeValue, newValue)
			return
		}
		if !alreadyExists {
			uniqueValueCount++
		}
	}
	t.Logf("Testing with %d total values, %d unique values", len(testData.values), uniqueValueCount)
	// validate the data structure
	iterator := testData.binaryTree.GetIterator()
	previousValue := -1
	for iterator.HasNextValue() {
		currentValue := iterator.GetNextValue()
		if currentValue <= previousValue {
			t.Errorf("values out of order: %d then %d", previousValue, currentValue)
			return
		}
		previousValue = currentValue
		if iterator.currentNode.weight < -1 || iterator.currentNode.weight > 1 {
			t.Errorf("found value %d with unbalanced weight %d", currentValue, iterator.currentNode.weight)
			return
		}
	}
	// ensure that search works well
	for _, value := range testData.values {
		returnValue, alreadyExists := testData.binaryTree.Insert(value, true)
		if returnValue != value || alreadyExists != true {
			t.Errorf("Could not find value %d in binary tree", value)
			return
		}
	}
}
