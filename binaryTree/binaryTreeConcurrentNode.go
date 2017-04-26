package binaryTree

import (
	"fmt"
	"sync"
)

// btpMutex a Mutex wrapper that helps to keep track of which process has a specific Mutex locked
type btpMutex struct {
	mutexID     string
	mutex       sync.Mutex
	lockCounter int
	currentLock int
	node        *binaryTreeConcurrentNode
}

// lock logs a message if the Mutex is already locked before locking the Mutex, and then logs when the Mutex is locked
func (mutex *btpMutex) lock(logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) {
	if mutex.currentLock > 0 {
		if logFunction("") {
			logFunction(fmt.Sprintf("mutex %s already locked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node, btom)))
		}
	}
	mutex.mutex.Lock()
	mutex.lockCounter++
	mutex.currentLock = mutex.lockCounter
	*mutexesLockedCounter++
	if logFunction("") {
		logFunction(fmt.Sprintf("mutex %s locked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node, btom)))
	}
}

// unlock logs a message after the Mutex is unlocked
func (mutex *btpMutex) unlock(logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) {
	mutex.currentLock = 0
	mutex.mutex.Unlock()
	*mutexesLockedCounter--
	if logFunction("") {
		logFunction(fmt.Sprintf("mutex %s unlocked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node, btom)))
	}
}

// binaryTreeConcurrentNode creates a weight-balanced concurrent binary tree that supports parallel insert, delete and search processes
type binaryTreeConcurrentNode struct {
	mutex            btpMutex
	valueIndex       int
	weightMutex      btpMutex
	currentWeight    int
	possibleWtAdjust [2]int // possible weight adjustments pending inserts and deletions
	parent           *binaryTreeConcurrentNode
	sideFromParent   int
	leftright        [2]*binaryTreeConcurrentNode
	branchBoundaries [2]int
	branchBMutices   [2]btpMutex
	rebalancing      bool
}

// BinaryTreeOperationManager interface required by binary tree operations to store and compare values
type BinaryTreeOperationManager interface {
	StoreValue() int                 // stores the operation value, can be called multiple times and should return the same index
	RestoreValue() int               // copies the value to a new location and returns the index
	DeleteValue()                    // deletes value from the storage
	CompareValueTo(int) int          // compares set value to the stored value
	GetValueString() string          // gets a string for the operation value
	GetStoredValueString(int) string // gets a string for the stored value
	SetValueToStoredValue(int)       // sets the value to a stored value, allows reuse
	HandleResult(int, bool)          // handles the result of the operation
}

// BtpLogFunction A log function that returns true if logging is turned on
// when a function is passed as the parameter it should be called to product the string to be logged when logging is turned on
type BtpLogFunction func(string) bool

// btpSetLogFunction Takes a btpLogFunction and wraps it in a new one if logging is turned on
// the new function will insert the id string in front of any messages
func btpSetLogFunction(oldLogFunction BtpLogFunction, id string) (newLogFunction BtpLogFunction) {
	if !oldLogFunction("") {
		return oldLogFunction
	}
	newLogFunction = func(logString string) (isImplemented bool) {
		if len(logString) != 0 {
			oldLogFunction(fmt.Sprintf("%s %s", id, logString))
		}
		return true
	}
	return
}

// btpGetWeight returns a node's weight value, but locks the weight mutex first, and unlocks it when it's done
func btpGetWeight(node *binaryTreeConcurrentNode, logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) (currentWeight int) {
	node.weightMutex.lock(logFunction, mutexesLockedCounter, btom)
	currentWeight = node.currentWeight
	node.weightMutex.unlock(logFunction, mutexesLockedCounter, btom)
	return
}

// btpAdjustPossibleWtAdj Locks the weight mutex and then adjusts one of the possible weight adjustment values by the given amount
// it unlocks the weight mutex when it's done
func btpAdjustPossibleWtAdj(node *binaryTreeConcurrentNode, side int, amount int, logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) {
	node.weightMutex.lock(logFunction, mutexesLockedCounter, btom)
	node.possibleWtAdjust[side] += amount
	node.weightMutex.unlock(logFunction, mutexesLockedCounter, btom)
}

// btpAdjustWeightAndPossibleWtAdj Locks the weight mutex, adjusts weight values, and then unlocks the weight mutex when it's done
// weight is adjusted by given amount and then corresponding possible weight adjustment value is decreased
func btpAdjustWeightAndPossibleWtAdj(node *binaryTreeConcurrentNode, amount int, logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) {
	node.weightMutex.lock(logFunction, mutexesLockedCounter, btom)
	defer node.weightMutex.unlock(logFunction, mutexesLockedCounter, btom)
	node.currentWeight += amount
	if amount > 0 {
		node.possibleWtAdjust[1] -= amount
		if node.possibleWtAdjust[1] < 0 {
			panic("positive possible weight adjustment value should never drop below 0")
		}
	} else {
		node.possibleWtAdjust[0] += amount
		if node.possibleWtAdjust[0] < 0 {
			panic("negative possible weight adjustment value should never drop below 0")
		}
	}
}

func btpGetNodePath(node *binaryTreeConcurrentNode) string {
	if node.parent != nil {
		return fmt.Sprint(btpGetNodePath(node.parent), node.sideFromParent)
	}
	return fmt.Sprint(node.sideFromParent)
}

// btpGetNodeString returns a string representation of the node used for logging
func btpGetNodeString(node *binaryTreeConcurrentNode, btom BinaryTreeOperationManager) string {
	if node == nil {
		return "nil node"
	}
	branchBoundaryStrings := [2]string{"nil", "nil"}
	if node.branchBoundaries[0] > -1 {
		branchBoundaryStrings[0] = btom.GetStoredValueString(node.branchBoundaries[0])
	}
	if node.branchBoundaries[1] > -1 {
		branchBoundaryStrings[1] = btom.GetStoredValueString(node.branchBoundaries[1])
	}
	return fmt.Sprintf("btp %s, value %s, parent %s, left %s, right %s, branch bounds %s - %s, weight %d, possible weight mods -%d +%d",
		btpGetNodePath(node), btpGetValue(node, btom), btpGetValue(node.parent, btom), btpGetValue(node.leftright[0], btom), btpGetValue(node.leftright[1], btom),
		branchBoundaryStrings[0], branchBoundaryStrings[1], node.currentWeight, node.possibleWtAdjust[0], node.possibleWtAdjust[1])
}

// btpGetBranchBoundary locks the Mutex, returns the value, and unlocks the Mutex for the corresponding boundary side
func btpGetBranchBoundary(node *binaryTreeConcurrentNode, side int, logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) (valueIndex int) {
	node.branchBMutices[side].lock(logFunction, mutexesLockedCounter, btom)
	valueIndex = node.branchBoundaries[side]
	node.branchBMutices[side].unlock(logFunction, mutexesLockedCounter, btom)
	return
}

func btpNextStep(node *binaryTreeConcurrentNode, comparisonResult int, logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) (nextNode *binaryTreeConcurrentNode, matchFound bool) {
	switch comparisonResult {
	case -1:
		nextNode = node.leftright[0]
	case 1:
		nextNode = node.leftright[1]
	case 0:
		nextNode = node
		matchFound = true
	default:
		panic(fmt.Sprintf("compareValues function returned invalid comparison value %d", comparisonResult))
	}
	if comparisonResult != 0 {
		nextNode.mutex.lock(logFunction, mutexesLockedCounter, btom)
		node.mutex.unlock(logFunction, mutexesLockedCounter, btom)
	}
	return
}

// btpSearch Returns the value if it is found, or nil if the value was not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpSearch(node *binaryTreeConcurrentNode, onFirstLock func(), logFunction BtpLogFunction, btom BinaryTreeOperationManager) {
	semaphoreLockCount := 0
	if node == nil {
		logFunction("BTPSearch should not be called with a nil binaryTree value")
		panic("BTPSearch should not be called with a nil binaryTree value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPSearch ", btom.GetValueString()))
	}
	node.mutex.lock(logFunction, &semaphoreLockCount, btom) // lock the current tree
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	for node.valueIndex > -1 && !matchFound {
		node, matchFound = btpNextStep(node, btom.CompareValueTo(node.valueIndex), logFunction, &semaphoreLockCount, btom)
	}
	btom.HandleResult(node.valueIndex, matchFound)
	node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	if semaphoreLockCount > 0 {
		logFunction("BTPSearch did not release all of it's locks")
		panic("BTPSearch did not release all of it's locks")
	}
}

type btpAdjustNodeElement struct {
	node *binaryTreeConcurrentNode
	side int
}

// btpInsert Returns the value to be inserted, and wether a match was found with an existing value and thus no insertion was required.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpInsert(node *binaryTreeConcurrentNode, btom BinaryTreeOperationManager, onFirstLock func(), onRebalance func(), logFunction BtpLogFunction) {
	semaphoreLockCount := 0
	if node == nil {
		logFunction("BTPInsert should not be called with a nil binaryTree value")
		panic("BTPInsert should not be called with a nil binaryTree value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPInsert ", btom.GetValueString()))
	}
	adjustWeights := []btpAdjustNodeElement{}
	node.mutex.lock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	for node.valueIndex > -1 && !matchFound {
		comparisonResult := btom.CompareValueTo(node.valueIndex)
		if comparisonResult != 0 {
			sideIndex := 0
			if comparisonResult > 0 {
				sideIndex = 1
			}
			btpAdjustPossibleWtAdj(node, sideIndex, 1, logFunction, &semaphoreLockCount, btom)
			adjustWeights = append(adjustWeights, btpAdjustNodeElement{node: node, side: sideIndex})
			if btom.CompareValueTo(node.branchBoundaries[sideIndex]) == comparisonResult {
				node.branchBoundaries[sideIndex] = btom.StoreValue()
			}
			node.leftright[sideIndex].branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
			node.leftright[sideIndex].branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
			node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
			node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
		}
		node, matchFound = btpNextStep(node, comparisonResult, logFunction, &semaphoreLockCount, btom)
	}
	if node.valueIndex == -1 {
		node.valueIndex = btom.StoreValue()
		node.branchBoundaries = [2]int{node.valueIndex, node.valueIndex}
		node.leftright[0] = &binaryTreeConcurrentNode{parent: node, valueIndex: -1, branchBoundaries: [2]int{-1, -1}, sideFromParent: 0}
		node.leftright[1] = &binaryTreeConcurrentNode{parent: node, valueIndex: -1, branchBoundaries: [2]int{-1, -1}, sideFromParent: 1}
		node.branchBMutices[0].mutexID = "branchBoundary0"
		node.branchBMutices[1].mutexID = "branchBoundary1"
		node.mutex.mutexID = "node"
		node.weightMutex.mutexID = "weight"
		node.branchBMutices[0].node = node
		node.branchBMutices[1].node = node
		node.weightMutex.node = node
		node.mutex.node = node
	}
	node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
	resultIndex := node.valueIndex
	node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	btom.HandleResult(resultIndex, matchFound)
	for _, adjustWeight := range adjustWeights {
		if !matchFound {
			btpAdjustWeightAndPossibleWtAdj(adjustWeight.node, adjustWeight.side*2-1, logFunction, &semaphoreLockCount, btom)
		} else {
			btpAdjustPossibleWtAdj(adjustWeight.node, adjustWeight.side, -1, logFunction, &semaphoreLockCount, btom)
		}
		if logFunction("") {
			logFunction(fmt.Sprintf("adjusting weights %s", btpGetNodeString(adjustWeight.node, btom)))
		}
		btpRebalance(adjustWeight.node, onRebalance, logFunction, btom)
	}
	if semaphoreLockCount > 0 {
		if logFunction("") {
			logFunction("BTPInsert did not release all of it's locks")
		}
		panic("BTPInsert did not release all of it's locks")
	}
}

// btpDelete Deletes the given value from the given tree.
// Throws a panic if mustMatch is set to true and a matching value is not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpDelete(node *binaryTreeConcurrentNode, btom BinaryTreeOperationManager, onFirstLock func(), onRebalance func(), mustMatch bool, logFunction BtpLogFunction) {
	semaphoreLockCount := 0
	if node == nil {
		logFunction("BTPDelete should not be called with a nil node value")
		panic("BTPDelete should not be called with a nil node value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPDelete", btom.GetValueString()))
	}
	adjustWeights := []btpAdjustNodeElement{}
	adjustChildBounds := []btpAdjustNodeElement{}
	node.mutex.lock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	var closestValues [2]int
	for node.valueIndex > -1 && !matchFound {
		comparisonResult := btom.CompareValueTo(node.valueIndex)
		if comparisonResult != 0 {
			sideToDeleteFrom := 0
			if comparisonResult > 0 {
				sideToDeleteFrom = 1
			}
			// adjust weights
			btpAdjustPossibleWtAdj(node, 1-sideToDeleteFrom, 1, logFunction, &semaphoreLockCount, btom)
			adjustWeights = append(adjustWeights, btpAdjustNodeElement{node: node, side: 1 - sideToDeleteFrom})
			// check if branchBounds need to be adjusted
			if btom.CompareValueTo(node.branchBoundaries[sideToDeleteFrom]) == 0 {
				mustMatch = true
				adjustChildBounds = append(adjustChildBounds, btpAdjustNodeElement{node: node, side: sideToDeleteFrom})
			} else {
				node.branchBMutices[sideToDeleteFrom].unlock(logFunction, &semaphoreLockCount, btom)
			}
			// adjust closestValues
			closestValues[1-sideToDeleteFrom] = node.valueIndex
			node.leftright[sideToDeleteFrom].branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
			node.leftright[sideToDeleteFrom].branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
			node.branchBMutices[1-sideToDeleteFrom].unlock(logFunction, &semaphoreLockCount, btom)
		}
		node, matchFound = btpNextStep(node, comparisonResult, logFunction, &semaphoreLockCount, btom)
	}
	var nodeToDeleteFrom *binaryTreeConcurrentNode
	if matchFound {
		node.leftright[0].mutex.lock(logFunction, &semaphoreLockCount, btom)
		node.leftright[1].mutex.lock(logFunction, &semaphoreLockCount, btom)
		// adjust closest values
		if node.leftright[0].valueIndex > -1 {
			closestValues[0] = btpGetBranchBoundary(node.leftright[0], 1, logFunction, &semaphoreLockCount, btom)
		}
		if node.leftright[1].valueIndex > -1 {
			closestValues[1] = btpGetBranchBoundary(node.leftright[1], 0, logFunction, &semaphoreLockCount, btom)
		}
		for _, nodeAndSide := range adjustChildBounds {
			nodeAndSide.node.branchBoundaries[nodeAndSide.side] = closestValues[1-nodeAndSide.side]
			nodeAndSide.node.branchBMutices[nodeAndSide.side].unlock(logFunction, &semaphoreLockCount, btom)
			if logFunction("") {
				logFunction(fmt.Sprintf("adjusted boundaries %s", btpGetNodeString(nodeAndSide.node, btom)))
			}
		}
		// remove it
		if node.leftright[0].valueIndex == -1 && node.leftright[1].valueIndex == -1 {
			node.leftright[0].mutex.unlock(logFunction, &semaphoreLockCount, btom)
			node.leftright[1].mutex.unlock(logFunction, &semaphoreLockCount, btom)
			node.valueIndex = -1
			node.leftright[0] = nil
			node.leftright[1] = nil
			node.branchBoundaries[0] = -1
			node.branchBoundaries[1] = -1
			if logFunction("") {
				logFunction(fmt.Sprintf("deleted leaf %s", btpGetNodeString(node, btom)))
			}
		} else {
			sideToDeleteFrom := -1
			for sideToDeleteFrom < 0 {
				node.weightMutex.lock(logFunction, &semaphoreLockCount, btom)
				if node.currentWeight-node.possibleWtAdjust[0] > -1 {
					node.currentWeight--
					sideToDeleteFrom = 1
				} else if node.currentWeight+node.possibleWtAdjust[1] < 1 {
					node.currentWeight++
					sideToDeleteFrom = 0
				}
				node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)
			}
			// update with new value
			node.valueIndex = btpGetBranchBoundary(node.leftright[sideToDeleteFrom], 1-sideToDeleteFrom, logFunction, &semaphoreLockCount, btom)
			if node.valueIndex == -1 {
				logFunction("Delete should not replace a deleted value that has one or more branches with a nil value")
				panic("Delete should not replace a deleted value that has one or more branches with a nil value")
			}
			// update branch boundary if old value was one of them
			if node.leftright[1-sideToDeleteFrom].valueIndex == -1 {
				node.branchBoundaries[1-sideToDeleteFrom] = node.valueIndex
			}
			node.leftright[0].mutex.unlock(logFunction, &semaphoreLockCount, btom)
			node.leftright[1].mutex.unlock(logFunction, &semaphoreLockCount, btom)
			if logFunction("") {
				logFunction(fmt.Sprintf("deleted branching node %s", btpGetNodeString(node, btom)))
			}
			nodeToDeleteFrom = node.leftright[sideToDeleteFrom]
		}
		btom.DeleteValue()
	} else {
		for _, nodeAndSide := range adjustChildBounds {
			nodeAndSide.node.branchBMutices[nodeAndSide.side].unlock(logFunction, &semaphoreLockCount, btom)
		}
		logFunction("node to delete not found")
	}
	node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
	if nodeToDeleteFrom != nil {
		// delete new value from old location in new process
		btom.SetValueToStoredValue(node.valueIndex)
		btpDelete(nodeToDeleteFrom, btom, func() {
			node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
		}, onRebalance, true, logFunction)
	} else {
		node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	}
	btom.HandleResult(-1, matchFound)
	for _, adjustWeight := range adjustWeights {
		if matchFound {
			btpAdjustWeightAndPossibleWtAdj(adjustWeight.node, adjustWeight.side*2-1, logFunction, &semaphoreLockCount, btom)
		} else {
			btpAdjustPossibleWtAdj(adjustWeight.node, adjustWeight.side, -1, logFunction, &semaphoreLockCount, btom)
		}
		if logFunction("") {
			logFunction(fmt.Sprintf("adjusting weights %s", btpGetNodeString(adjustWeight.node, btom)))
		}
		btpRebalance(adjustWeight.node, onRebalance, logFunction, btom)
	}
	if !matchFound && mustMatch {
		logFunction("Failed to delete when value was known to exist")
		panic("Failed to delete when value was known to exist")
	}
	if semaphoreLockCount > 0 {
		logFunction("BTPDelete did not release all of it's locks")
		panic("BTPDelete did not release all of it's locks")
	}
}

// btpRebalance Can run in it's own goroutine and will not have a guaranteed start time in regards to parallel searches, inserts and deletions
func btpRebalance(node *binaryTreeConcurrentNode, onRebalance func(), logFunction BtpLogFunction, btom BinaryTreeOperationManager) {
	semaphoreLockCount := 0
	if node == nil {
		logFunction("btpRebalance called on a nil value")
		panic("btpRebalance called on a nil value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPRebalance", btom.GetStoredValueString(node.valueIndex)))
	}
	node.mutex.lock(logFunction, &semaphoreLockCount, btom)
	node.weightMutex.lock(logFunction, &semaphoreLockCount, btom)
	if onRebalance != nil {
		onRebalance()
	}
	for node.currentWeight+node.possibleWtAdjust[1] < -1 || node.currentWeight-node.possibleWtAdjust[0] > 1 {
		newRootSide := 0            // side from which the new root node is being taken from
		if node.currentWeight > 0 { // swap sides if the weight is positive
			newRootSide = 1
		}
		rootNewSide := 1 - newRootSide // side where the old root node is being moved to

		// get the new root value
		newRootValueIndex := btpGetBranchBoundary(node.leftright[newRootSide], rootNewSide, logFunction, &semaphoreLockCount, btom)
		if newRootValueIndex == -1 {
			node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
			node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)
			logFunction("BTPRebalance should not replace root's value with a nil value")
			panic("BTPRebalance should not replace root's value with a nil value")
		}

		// adjust the binaryTree
		valueIndexToInsert := node.valueIndex
		btom.SetValueToStoredValue(newRootValueIndex)
		node.valueIndex = btom.RestoreValue()
		node.branchBMutices[newRootSide].lock(logFunction, &semaphoreLockCount, btom)
		if node.branchBoundaries[newRootSide] == newRootValueIndex {
			node.branchBoundaries[newRootSide] = node.valueIndex
		}
		node.branchBMutices[newRootSide].unlock(logFunction, &semaphoreLockCount, btom)
		node.currentWeight += 4*rootNewSide - 2

		// insert the oldRootValue on the rootNewSide
		btom.SetValueToStoredValue(valueIndexToInsert)
		btpInsert(node.leftright[rootNewSide], btom, func() {}, onRebalance, logFunction)

		// delete the newRootValue from the newRootSide
		btom.SetValueToStoredValue(newRootValueIndex)
		btpDelete(node.leftright[newRootSide], btom, func() {}, onRebalance, true, logFunction)
	}
	node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)
	node.rebalancing = false
	node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	if semaphoreLockCount > 0 {
		logFunction("btpRebalance did not release all of it's locks")
		panic("btpRebalance did not release all of it's locks")
	}
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func btpGetValue(node *binaryTreeConcurrentNode, btom BinaryTreeOperationManager) string {
	if node != nil && node.valueIndex > -1 {
		return btom.GetStoredValueString(node.valueIndex)
	}
	return "nil"
}

// btpGetNext returns the next BinaryTreeParrallel object, returns nil when it reaches the end
// not safe while values are being concurrently inserted
func btpGetNext(node *binaryTreeConcurrentNode) *binaryTreeConcurrentNode {
	if node == nil || node.valueIndex == -1 {
		return nil
	}
	if node.leftright[1].valueIndex != -1 {
		node = node.leftright[1]
		for node.leftright[0].valueIndex != -1 {
			node = node.leftright[0]
		}
		return node
	}
	for node.parent != nil && node.parent.leftright[1].valueIndex == node.valueIndex {
		node = node.parent
	}
	return node.parent
}

// btpGetFirst returns the first value in the tree, or nil if the tree contains no values
// not safe while values are being concurrently inserted
func btpGetFirst(node *binaryTreeConcurrentNode) *binaryTreeConcurrentNode {
	if node == nil || node.valueIndex == -1 {
		return nil
	}
	for node.parent != nil {
		node = node.parent
	}
	for node.leftright[0].valueIndex != -1 {
		node = node.leftright[0]
	}
	return node
}
