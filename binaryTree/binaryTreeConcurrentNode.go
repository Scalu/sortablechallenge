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
	leftright        [2]*binaryTreeConcurrentNode
	branchBoundaries [2]int
	branchBMutices   [2]btpMutex
	rebalancing      bool
}

// BinaryTreeOperationManager interface required by binary tree operations to store and compare values
type BinaryTreeOperationManager interface {
	StoreValue() int                                     // stores the operation value, can be called multiple times and should return the same index
	UpdateValue(int)                                     // updates the value with the operation value
	DeleteValue(int)                                     // deletes a value from the storage
	CompareValueTo(int) int                              // compares set value to the stored value
	GetValueString() string                              // gets a string for the operation value
	GetStoredValueString(int) string                     // gets a string for the stored value
	CloneWithStoredValue(int) BinaryTreeOperationManager // returns a copy of this with a new value from storage for generating new insert/delete/rebalance tasks
	LaunchNewProcess(func())                             // launches a new process (rebalance, insert or delete)
	HandleResult(int, bool)                              // handles the result of the operation
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

// btpRebalanceIfNecessary If a branch is unbalanced, it launches a new goroutine to rebalance that branch
// it will not try to rebalance a branch that is being rebalanced, or that could possibly be balanced pending insert and delete adjustments
func btpRebalanceIfNecessary(binaryTree *binaryTreeConcurrentNode, onRebalance func(), logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) {
	binaryTree.weightMutex.lock(logFunction, mutexesLockedCounter, btom)
	if !binaryTree.rebalancing &&
		(binaryTree.currentWeight+binaryTree.possibleWtAdjust[1] < -1 ||
			binaryTree.currentWeight-binaryTree.possibleWtAdjust[0] > 1) {
		binaryTree.rebalancing = true
		btom.LaunchNewProcess(func() {
			btpRebalance(binaryTree, onRebalance, logFunction, btom)
		})
	}
	binaryTree.weightMutex.unlock(logFunction, mutexesLockedCounter, btom)
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
	return fmt.Sprintf("btp %s, parent %s, left %s, right %s, branch bounds %s - %s, weight %d, possible weight mods -%d +%d",
		btpGetValue(node, btom), btpGetValue(node.parent, btom), btpGetValue(node.leftright[0], btom), btpGetValue(node.leftright[1], btom),
		branchBoundaryStrings[0], branchBoundaryStrings[1], node.currentWeight, node.possibleWtAdjust[0], node.possibleWtAdjust[1])
}

// btpGetBranchBoundary locks the Mutex, returns the value, and unlocks the Mutex for the corresponding boundary side
func btpGetBranchBoundary(node *binaryTreeConcurrentNode, side int, logFunction BtpLogFunction, mutexesLockedCounter *int, btom BinaryTreeOperationManager) (valueIndex int) {
	node.branchBMutices[side].lock(logFunction, mutexesLockedCounter, btom)
	valueIndex = node.branchBoundaries[side]
	node.branchBMutices[side].unlock(logFunction, mutexesLockedCounter, btom)
	return
}

// btpStep steps through the binary tree nodes until a nil value is reached or the compareValues function returns a 0 to indicate a match with the value
// It keeps the current node locked until the next node is locked so that goroutines using btpStep cannot cross each other
func btpStep(binaryTree *binaryTreeConcurrentNode, compareValues func(*binaryTreeConcurrentNode) int, btom BinaryTreeOperationManager, logFunction BtpLogFunction, mutexesLockedCounter *int) (nextStep *binaryTreeConcurrentNode, matchFound bool) {
	if binaryTree == nil {
		return // defaults to nil, false
	}
	nextStep = binaryTree
	for binaryTree.valueIndex > -1 && !matchFound {
		comparisonResult := compareValues(binaryTree)
		switch comparisonResult {
		case -1:
			nextStep = binaryTree.leftright[0]
		case 1:
			nextStep = binaryTree.leftright[1]
		case 0:
			nextStep = binaryTree
			matchFound = true
		default:
			panic(fmt.Sprintf("compareValues function returned invalid comparison value %d", comparisonResult))
		}
		if comparisonResult != 0 {
			nextStep.mutex.lock(logFunction, mutexesLockedCounter, btom)
			binaryTree.mutex.unlock(logFunction, mutexesLockedCounter, btom)
		}
		binaryTree = nextStep
	}
	return
}

// btpSearch Returns the value if it is found, or nil if the value was not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpSearch(binaryTree *binaryTreeConcurrentNode, onFirstLock func(), logFunction BtpLogFunction, btom BinaryTreeOperationManager) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction("BTPSearch did not release all of it's locks")
			panic("BTPSearch did not release all of it's locks")
		}
	}()
	if binaryTree == nil {
		logFunction("BTPSearch should not be called with a nil binaryTree value")
		panic("BTPSearch should not be called with a nil binaryTree value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPSearch ", btom.GetValueString()))
	binaryTree.mutex.lock(logFunction, &semaphoreLockCount, btom) // lock the current tree
	defer func() {
		binaryTree.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	}()
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *binaryTreeConcurrentNode) int {
		return btom.CompareValueTo(binaryTree.valueIndex)
	}, btom, logFunction, &semaphoreLockCount)
	btom.HandleResult(binaryTree.valueIndex, matchFound)
}

// btpInsert Returns the value to be inserted, and wether a match was found with an existing value and thus no insertion was required.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpInsert(binaryTree *binaryTreeConcurrentNode, btom BinaryTreeOperationManager, onFirstLock func(), onRebalance func(), logFunction BtpLogFunction) {
	semaphoreLockCount := 0
	if binaryTree == nil {
		logFunction("BTPInsert should not be called with a nil binaryTree value")
		panic("BTPInsert should not be called with a nil binaryTree value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPInsert ", btom.GetValueString()))
	adjustWeights := func() {} // does nothing yet
	binaryTree.branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
	binaryTree.branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
	binaryTree.mutex.lock(logFunction, &semaphoreLockCount, btom)
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *binaryTreeConcurrentNode) (comparisonResult int) {
		comparisonResult = btom.CompareValueTo(binaryTree.valueIndex)
		if comparisonResult != 0 {
			sideIndex := 0
			if comparisonResult > 0 {
				sideIndex = 1
			}
			btpAdjustPossibleWtAdj(binaryTree, sideIndex, 1, logFunction, &semaphoreLockCount, btom)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				if !matchFound {
					btpAdjustWeightAndPossibleWtAdj(binaryTree, comparisonResult, logFunction, &semaphoreLockCount, btom)
				} else {
					btpAdjustPossibleWtAdj(binaryTree, sideIndex, -1, logFunction, &semaphoreLockCount, btom)
				}
				if logFunction("") {
					logFunction(fmt.Sprintf("adjusting weights %s", btpGetNodeString(binaryTree, btom)))
				}
				btpRebalanceIfNecessary(binaryTree, onRebalance, logFunction, &semaphoreLockCount, btom)
				prevAdjustWeights()
			}
			if btom.CompareValueTo(binaryTree.branchBoundaries[sideIndex]) == comparisonResult {
				binaryTree.branchBoundaries[sideIndex] = btom.StoreValue()
			}
			binaryTree.leftright[sideIndex].branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
			binaryTree.leftright[sideIndex].branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
			binaryTree.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
			binaryTree.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
		}
		return
	}, btom, logFunction, &semaphoreLockCount)
	if binaryTree.valueIndex == -1 {
		binaryTree.valueIndex = btom.StoreValue()
		binaryTree.branchBoundaries = [2]int{binaryTree.valueIndex, binaryTree.valueIndex}
		binaryTree.leftright[0] = &binaryTreeConcurrentNode{parent: binaryTree, valueIndex: -1, branchBoundaries: [2]int{-1, -1}}
		binaryTree.leftright[1] = &binaryTreeConcurrentNode{parent: binaryTree, valueIndex: -1, branchBoundaries: [2]int{-1, -1}}
		binaryTree.branchBMutices[0].mutexID = "branchBoundary0"
		binaryTree.branchBMutices[1].mutexID = "branchBoundary1"
		binaryTree.mutex.mutexID = "node"
		binaryTree.weightMutex.mutexID = "weight"
		binaryTree.branchBMutices[0].node = binaryTree
		binaryTree.branchBMutices[1].node = binaryTree
		binaryTree.weightMutex.node = binaryTree
		binaryTree.mutex.node = binaryTree
	}
	binaryTree.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
	binaryTree.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
	btom.HandleResult(binaryTree.valueIndex, matchFound)
	binaryTree.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	adjustWeights()
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
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction("BTPDelete did not release all of it's locks")
			panic("BTPDelete did not release all of it's locks")
		}
	}()
	if node == nil {
		logFunction("BTPDelete should not be called with a nil node value")
		panic("BTPDelete should not be called with a nil node value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPDelete", btom.GetValueString()))
	adjustWeights := func() {} // does nothing yet
	defer func() { adjustWeights() }()
	adjustChildBounds := func() {} // does nothing for now
	defer func() { adjustChildBounds() }()
	node.branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
	node.branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
	node.mutex.lock(logFunction, &semaphoreLockCount, btom)
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	var closestValues [2]int
	node, matchFound = btpStep(node, func(node *binaryTreeConcurrentNode) int {
		comparisonResult := btom.CompareValueTo(node.valueIndex)
		if comparisonResult != 0 {
			sideToDeleteFrom := 0
			if comparisonResult > 0 {
				sideToDeleteFrom = 1
			}
			// adjust weights
			btpAdjustPossibleWtAdj(node, 1-sideToDeleteFrom, 1, logFunction, &semaphoreLockCount, btom)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				prevAdjustWeights()
				if matchFound {
					btpAdjustWeightAndPossibleWtAdj(node, 0-comparisonResult, logFunction, &semaphoreLockCount, btom)
				} else {
					btpAdjustPossibleWtAdj(node, 1-sideToDeleteFrom, -1, logFunction, &semaphoreLockCount, btom)
				}
				if logFunction("") {
					logFunction(fmt.Sprintf("adjusted weights %s", btpGetNodeString(node, btom)))
				}
				btpRebalanceIfNecessary(node, onRebalance, logFunction, &semaphoreLockCount, btom)
			}
			// check if branchBounds need to be adjusted
			if btom.CompareValueTo(node.branchBoundaries[sideToDeleteFrom]) == 0 {
				mustMatch = true
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					prevAdjustChildBounds()
					node.branchBoundaries[sideToDeleteFrom] = closestValues[1-sideToDeleteFrom]
					node.branchBMutices[sideToDeleteFrom].unlock(logFunction, &semaphoreLockCount, btom)
					if logFunction("") {
						logFunction(fmt.Sprintf("adjusted boundaries %s", btpGetNodeString(node, btom)))
					}
				}
			} else {
				node.branchBMutices[sideToDeleteFrom].unlock(logFunction, &semaphoreLockCount, btom)
			}
			// adjust closestValues
			closestValues[1-sideToDeleteFrom] = node.valueIndex
			node.leftright[sideToDeleteFrom].branchBMutices[0].lock(logFunction, &semaphoreLockCount, btom)
			node.leftright[sideToDeleteFrom].branchBMutices[1].lock(logFunction, &semaphoreLockCount, btom)
			node.branchBMutices[1-sideToDeleteFrom].unlock(logFunction, &semaphoreLockCount, btom)
		}
		return comparisonResult
	}, btom, logFunction, &semaphoreLockCount)
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
		adjustChildBounds()
		adjustChildBounds = func() {}
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
			node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
			node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
			node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
		} else {
			sideToDeleteFrom := 0
			node.weightMutex.lock(logFunction, &semaphoreLockCount, btom)
			if node.currentWeight > 0 || node.currentWeight == 0 && node.possibleWtAdjust[1] > 0 {
				node.currentWeight--
				sideToDeleteFrom = 1
			} else {
				node.currentWeight++
			}
			node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)

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
			// delete new value from old location in new process
			btpDelete(node.leftright[sideToDeleteFrom], btom.CloneWithStoredValue(node.valueIndex), func() {
				node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
			}, onRebalance, true, logFunction)
			node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
			node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
			if logFunction("") {
				logFunction(fmt.Sprintf("deleted branching node %s", btpGetNodeString(node, btom)))
			}
		}
	} else if mustMatch {
		logFunction("Failed to delete when value was known to exist")
		node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
		node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
		node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
		panic("Failed to delete when value was known to exist")
	} else {
		logFunction("node to delete not found")
		node.branchBMutices[0].unlock(logFunction, &semaphoreLockCount, btom)
		node.branchBMutices[1].unlock(logFunction, &semaphoreLockCount, btom)
		node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	}
	btom.HandleResult(-1, matchFound)
}

// btpRebalance Can run in it's own goroutine and will not have a guaranteed start time in regards to parallel searches, inserts and deletions
func btpRebalance(node *binaryTreeConcurrentNode, onRebalance func(), logFunction BtpLogFunction, btom BinaryTreeOperationManager) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction("btpRebalance did not release all of it's locks")
			panic("btpRebalance did not release all of it's locks")
		}
	}()
	if node == nil || node.valueIndex == -1 {
		logFunction("btpRebalance called on a nil value")
		panic("btpRebalance called on a nil value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPRebalance", btom.GetStoredValueString(node.valueIndex)))
	node.mutex.lock(logFunction, &semaphoreLockCount, btom)
	node.weightMutex.lock(logFunction, &semaphoreLockCount, btom)
	if node.currentWeight+node.possibleWtAdjust[1] > -2 && node.currentWeight-node.possibleWtAdjust[0] < 2 {
		node.rebalancing = false
		node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
		node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)
		return
	}
	defer func() {
		node.rebalancing = false
		btpRebalanceIfNecessary(node, onRebalance, logFunction, &semaphoreLockCount, btom)
	}()
	if onRebalance != nil {
		onRebalance()
	}
	newRootSide := 0            // side from which the new root node is being taken from
	if node.currentWeight > 0 { // swap sides if the weight is positive
		newRootSide = 1
	}
	rootNewSide := 1 - newRootSide // side where the old root node is being moved to

	// get the new root value
	newRootValue := btpGetBranchBoundary(node.leftright[newRootSide], rootNewSide, logFunction, &semaphoreLockCount, btom)
	if newRootValue == -1 {
		node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
		node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)
		logFunction("BTPRebalance should not replace root's value with a nil value")
		panic("BTPRebalance should not replace root's value with a nil value")
	}

	// adjust the binaryTree
	valueToInsert := node.valueIndex
	node.valueIndex = newRootValue
	node.currentWeight += 4*rootNewSide - 2
	node.weightMutex.unlock(logFunction, &semaphoreLockCount, btom)

	// insert the oldRootValue on the rootNewSide
	btpInsert(node.leftright[rootNewSide], btom.CloneWithStoredValue(valueToInsert), func() {}, onRebalance, logFunction)

	// delete the newRootValue from the newRootSide
	btpDelete(node.leftright[newRootSide], btom.CloneWithStoredValue(newRootValue), func() {
		node.mutex.unlock(logFunction, &semaphoreLockCount, btom)
	}, onRebalance, true, logFunction)
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func btpGetValue(binaryTree *binaryTreeConcurrentNode, btom BinaryTreeOperationManager) string {
	if binaryTree != nil && binaryTree.valueIndex > -1 {
		return btom.GetStoredValueString(binaryTree.valueIndex)
	}
	return "nil"
}

// btpGetNext returns the next BinaryTreeParrallel object, returns nil when it reaches the end
// not safe while values are being concurrently inserted
func btpGetNext(binaryTree *binaryTreeConcurrentNode) *binaryTreeConcurrentNode {
	if binaryTree == nil || binaryTree.valueIndex == -1 {
		return nil
	}
	if binaryTree.leftright[1].valueIndex != -1 {
		binaryTree = binaryTree.leftright[1]
		for binaryTree.leftright[0].valueIndex != -1 {
			binaryTree = binaryTree.leftright[0]
		}
		return binaryTree
	}
	for binaryTree.parent != nil && binaryTree.parent.leftright[1].valueIndex == binaryTree.valueIndex {
		binaryTree = binaryTree.parent
	}
	return binaryTree.parent
}

// btpGetFirst returns the first value in the tree, or nil if the tree contains no values
// not safe while values are being concurrently inserted
func btpGetFirst(binaryTree *binaryTreeConcurrentNode) *binaryTreeConcurrentNode {
	if binaryTree == nil || binaryTree.valueIndex == -1 {
		return nil
	}
	for binaryTree.parent != nil {
		binaryTree = binaryTree.parent
	}
	for binaryTree.leftright[0].valueIndex != -1 {
		binaryTree = binaryTree.leftright[0]
	}
	return binaryTree
}
