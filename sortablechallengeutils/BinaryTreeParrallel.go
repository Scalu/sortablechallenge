package sortablechallengeutils

import (
	"fmt"
	"sync"
)

// BinaryTreeValue is the interface for values being stored into the binary tree
type BinaryTreeValue interface {
	// CompareTo should return -1 if value's order comes before that given in the argument, or 1 if after, or 0 if they are equal
	CompareTo(BinaryTreeValue) int
	// ToString returns a string representation of the value for when logging is enabled, or for panics
	ToString() string
}

// btpMutex a Mutex wrapper that helps to keep track of which process has a specific Mutex locked
type btpMutex struct {
	mutexID     string
	mutex       sync.Mutex
	lockCounter int
	currentLock int
	node        *BinaryTreeParallel
}

// Lock logs a message if the Mutex is already locked before locking the Mutex, and then logs when the Mutex is locked
func (mutex *btpMutex) Lock(logFunction btpLogFunction, mutexesLockedCounter *int) {
	if mutex.currentLock > 0 {
		logFunction(func() string {
			return fmt.Sprintf("mutex %s already locked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node))
		})
	}
	mutex.mutex.Lock()
	mutex.lockCounter++
	mutex.currentLock = mutex.lockCounter
	*mutexesLockedCounter++
	logFunction(func() string {
		return fmt.Sprintf("mutex %s locked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node))
	})
}

// Unlock logs a message after the Mutex is unlocked
func (mutex *btpMutex) Unlock(logFunction btpLogFunction, mutexesLockedCounter *int) {
	mutex.currentLock = 0
	mutex.mutex.Unlock()
	*mutexesLockedCounter--
	logFunction(func() string {
		return fmt.Sprintf("mutex %s unlocked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node))
	})
}

// BinaryTreeParallel creates a weight-balanced concurrent binary tree that supports parallel insert, delete and search processes
type BinaryTreeParallel struct {
	mutex            btpMutex
	value            BinaryTreeValue
	weightMutex      btpMutex
	currentWeight    int
	possibleWtAdjust [2]int // possible weight adjustments pending inserts and deletions
	parent           *BinaryTreeParallel
	leftright        [2]*BinaryTreeParallel
	branchBoundaries [2]BinaryTreeValue
	branchBMutices   [2]btpMutex
	rebalancing      bool
}

// btpLogFunction A log function that returns true if logging is turned on
// when a function is passed as the parameter it should be called to product the string to be logged when logging is turned on
type btpLogFunction func(func() string) bool

// btpSetLogFunction Takes a btpLogFunction and wraps it in a new one if logging is turned on
// the new function will insert the id string in front of any messages
func btpSetLogFunction(oldLogFunction btpLogFunction, id string) (newLogFunction btpLogFunction) {
	if !oldLogFunction(nil) {
		return oldLogFunction
	}
	newLogFunction = func(getLogString func() string) (isImplemented bool) {
		if getLogString != nil {
			oldLogFunction(func() string { return fmt.Sprintf("%s %s", id, getLogString()) })
		}
		return true
	}
	return
}

// btpGetWeight returns a node's weight value, but locks the weight mutex first, and unlocks it when it's done
func btpGetWeight(node *BinaryTreeParallel, logFunction btpLogFunction, mutexesLockedCounter *int) int {
	node.weightMutex.Lock(logFunction, mutexesLockedCounter)
	defer node.weightMutex.Unlock(logFunction, mutexesLockedCounter)
	return node.currentWeight
}

// btpAdjustPossibleWtAdj Locks the weight mutex and then adjusts one of the possible weight adjustment values by the given amount
// it unlocks the weight mutex when it's done
func btpAdjustPossibleWtAdj(node *BinaryTreeParallel, side int, amount int, logFunction btpLogFunction, mutexesLockedCounter *int) {
	node.weightMutex.Lock(logFunction, mutexesLockedCounter)
	defer node.weightMutex.Unlock(logFunction, mutexesLockedCounter)
	node.possibleWtAdjust[side] += amount
}

// btpAdjustWeightAndPossibleWtAdj Locks the weight mutex, adjusts weight values, and then unlocks the weight mutex when it's done
// weight is adjusted by given amount and then corresponding possible weight adjustment value is decreased
func btpAdjustWeightAndPossibleWtAdj(node *BinaryTreeParallel, amount int, logFunction btpLogFunction, mutexesLockedCounter *int) {
	node.weightMutex.Lock(logFunction, mutexesLockedCounter)
	defer node.weightMutex.Unlock(logFunction, mutexesLockedCounter)
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
func btpRebalanceIfNecessary(binaryTree *BinaryTreeParallel, onRebalance func(), logFunction btpLogFunction, processCounterChannel chan int, mutexesLockedCounter *int) {
	binaryTree.weightMutex.Lock(logFunction, mutexesLockedCounter)
	defer binaryTree.weightMutex.Unlock(logFunction, mutexesLockedCounter)
	if !binaryTree.rebalancing &&
		(binaryTree.currentWeight+binaryTree.possibleWtAdjust[1] < -1 ||
			binaryTree.currentWeight-binaryTree.possibleWtAdjust[0] > 1) {
		binaryTree.rebalancing = true
		processCounterChannel <- 1
		go func() {
			defer func() { processCounterChannel <- -1 }()
			btpRebalance(binaryTree, onRebalance, logFunction, processCounterChannel)
		}()
	}

}

// btpGetNodeString returns a string representation of the node used for logging
func btpGetNodeString(node *BinaryTreeParallel) string {
	if node == nil {
		return "nil node"
	}
	branchBoundaryStrings := [2]string{"nil", "nil"}
	if node.branchBoundaries[0] != nil {
		branchBoundaryStrings[0] = node.branchBoundaries[0].ToString()
	}
	if node.branchBoundaries[1] != nil {
		branchBoundaryStrings[1] = node.branchBoundaries[1].ToString()
	}
	return fmt.Sprintf("btp %s, parent %s, left %s, right %s, branch bounds %s - %s, weight %d, possible weight mods -%d +%d",
		btpGetValue(node), btpGetValue(node.parent), btpGetValue(node.leftright[0]), btpGetValue(node.leftright[1]),
		branchBoundaryStrings[0], branchBoundaryStrings[1], node.currentWeight, node.possibleWtAdjust[0], node.possibleWtAdjust[1])
}

// btpGetBranchBoundary locks the Mutex, returns the value, and unlocks the Mutex for the corresponding boundary side
func btpGetBranchBoundary(node *BinaryTreeParallel, side int, logFunction btpLogFunction, mutexesLockedCounter *int) BinaryTreeValue {
	node.branchBMutices[side].Lock(logFunction, mutexesLockedCounter)
	defer node.branchBMutices[side].Unlock(logFunction, mutexesLockedCounter)
	return node.branchBoundaries[side]
}

// btpStep steps through the binary tree nodes until a nil value is reached or the compareValues function returns a 0 to indicate a match with the value
// It keeps the current node locked until the next node is locked so that goroutines using btpStep cannot cross each other
func btpStep(binaryTree *BinaryTreeParallel, compareValues func(*BinaryTreeParallel, BinaryTreeValue) int, value BinaryTreeValue, logFunction btpLogFunction, mutexesLockedCounter *int) (nextStep *BinaryTreeParallel, matchFound bool) {
	if binaryTree == nil {
		return // defaults to nil, false
	}
	nextStep = binaryTree
	for binaryTree.value != nil && !matchFound {
		comparisonResult := compareValues(binaryTree, value)
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
			nextStep.mutex.Lock(logFunction, mutexesLockedCounter)
			binaryTree.mutex.Unlock(logFunction, mutexesLockedCounter)
		}
		binaryTree = nextStep
	}
	return
}

// BTPSearch Returns the value if it is found, or nil if the value was not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func BTPSearch(binaryTree *BinaryTreeParallel, value BinaryTreeValue, previousLock *sync.Mutex, logFunction btpLogFunction) BinaryTreeValue {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction(func() string { return "BTPSearch did not release all of it's locks" })
			panic("BTPSearch did not release all of it's locks")
		}
	}()
	if binaryTree == nil {
		logFunction(func() string { return "BTPSearch should not be called with a nil binaryTree value" })
		panic("BTPSearch should not be called with a nil binaryTree value")
	}
	if value == nil {
		logFunction(func() string { return "BTPSearch should not search for a nil value" })
		panic("BTPSearch should not search for a nil value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPSearch", value.ToString()))
	binaryTree.mutex.Lock(logFunction, &semaphoreLockCount) // lock the current tree
	defer func() {
		binaryTree.mutex.Unlock(logFunction, &semaphoreLockCount)
	}()
	if previousLock != nil {
		previousLock.Unlock()
	}
	var matchFound bool
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value BinaryTreeValue) int {
		return value.CompareTo(binaryTree.value)
	}, value, logFunction, &semaphoreLockCount)
	if matchFound {
		return binaryTree.value
	}
	return nil
}

// BTPInsert Returns the value to be inserted, and wether a match was found with an existing value and thus no insertion was required.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func BTPInsert(binaryTree *BinaryTreeParallel, value BinaryTreeValue, previousLock *sync.Mutex, onRebalance func(), logFunction btpLogFunction, processCounterChannel chan int) (valueInserted BinaryTreeValue, matchFound bool) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction(func() string { return "BTPInsert did not release all of it's locks" })
			panic("BTPInsert did not release all of it's locks")
		}
	}()
	if binaryTree == nil {
		logFunction(func() string { return "BTPInsert should not be called with a nil binaryTree value" })
		panic("BTPInsert should not be called with a nil binaryTree value")
	}
	if value == nil {
		logFunction(func() string { return "BTPInsert should not try to insert a nil value" })
		panic("BTPInsert should not try to insert a nil value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPInsert", value.ToString()))
	adjustWeights := func() {} // does nothing yet
	defer func() { adjustWeights() }()
	binaryTree.branchBMutices[0].Lock(logFunction, &semaphoreLockCount)
	binaryTree.branchBMutices[1].Lock(logFunction, &semaphoreLockCount)
	binaryTree.mutex.Lock(logFunction, &semaphoreLockCount)
	defer func() {
		binaryTree.branchBMutices[0].Unlock(logFunction, &semaphoreLockCount)
		binaryTree.branchBMutices[1].Unlock(logFunction, &semaphoreLockCount)
		binaryTree.mutex.Unlock(logFunction, &semaphoreLockCount)
	}()
	if previousLock != nil {
		previousLock.Unlock()
		previousLock = nil
	}
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value BinaryTreeValue) (comparisonResult int) {
		comparisonResult = value.CompareTo(binaryTree.value)
		if comparisonResult != 0 {
			sideIndex := 0
			if comparisonResult > 0 {
				sideIndex = 1
			}
			btpAdjustPossibleWtAdj(binaryTree, sideIndex, 1, logFunction, &semaphoreLockCount)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				if !matchFound {
					btpAdjustWeightAndPossibleWtAdj(binaryTree, comparisonResult, logFunction, &semaphoreLockCount)
				} else {
					btpAdjustPossibleWtAdj(binaryTree, sideIndex, -1, logFunction, &semaphoreLockCount)
				}
				logFunction(func() string { return fmt.Sprintf("adjusting weights %s", btpGetNodeString(binaryTree)) })
				btpRebalanceIfNecessary(binaryTree, onRebalance, logFunction, processCounterChannel, &semaphoreLockCount)
				prevAdjustWeights()
			}
			if value.CompareTo(binaryTree.branchBoundaries[sideIndex]) == comparisonResult {
				binaryTree.branchBoundaries[sideIndex] = value
			}
			binaryTree.leftright[sideIndex].branchBMutices[0].Lock(logFunction, &semaphoreLockCount)
			binaryTree.leftright[sideIndex].branchBMutices[1].Lock(logFunction, &semaphoreLockCount)
			binaryTree.branchBMutices[0].Unlock(logFunction, &semaphoreLockCount)
			binaryTree.branchBMutices[1].Unlock(logFunction, &semaphoreLockCount)
		}
		return
	}, value, logFunction, &semaphoreLockCount)
	if binaryTree.value == nil {
		binaryTree.value = value
		binaryTree.branchBoundaries[0] = value
		binaryTree.branchBoundaries[1] = value
		binaryTree.leftright[0] = &BinaryTreeParallel{parent: binaryTree}
		binaryTree.leftright[1] = &BinaryTreeParallel{parent: binaryTree}
		binaryTree.branchBMutices[0].mutexID = "branchBoundary0"
		binaryTree.branchBMutices[1].mutexID = "branchBoundary1"
		binaryTree.mutex.mutexID = "node"
		binaryTree.weightMutex.mutexID = "weight"
		binaryTree.branchBMutices[0].node = binaryTree
		binaryTree.branchBMutices[1].node = binaryTree
		binaryTree.weightMutex.node = binaryTree
		binaryTree.mutex.node = binaryTree
	}
	return binaryTree.value, matchFound
}

// BTPDelete Deletes the given value from the given tree.
// Throws a panic if mustMatch is set to true and a matching value is not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func BTPDelete(node *BinaryTreeParallel, value BinaryTreeValue, previousLock *sync.Mutex, onRebalance func(), mustMatch bool, logFunction btpLogFunction, processCounterChannel chan int) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction(func() string { return "BTPDelete did not release all of it's locks" })
			panic("BTPDelete did not release all of it's locks")
		}
	}()
	if node == nil {
		logFunction(func() string { return "BTPDelete should not be called with a nil node value" })
		panic("BTPDelete should not be called with a nil node value")
	}
	if value == nil {
		logFunction(func() string { return "BTPDelete should not try to delete a nil value" })
		panic("BTPDelete should not try to delete a nil value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPDelete", value.ToString()))
	adjustWeights := func() {} // does nothing yet
	defer func() { adjustWeights() }()
	adjustChildBounds := func() {} // does nothing for now
	defer func() { adjustChildBounds() }()
	node.branchBMutices[0].Lock(logFunction, &semaphoreLockCount)
	node.branchBMutices[1].Lock(logFunction, &semaphoreLockCount)
	node.mutex.Lock(logFunction, &semaphoreLockCount)
	defer func() {
		node.branchBMutices[0].Unlock(logFunction, &semaphoreLockCount)
		node.branchBMutices[1].Unlock(logFunction, &semaphoreLockCount)
		node.mutex.Unlock(logFunction, &semaphoreLockCount)
	}()
	if previousLock != nil {
		previousLock.Unlock()
		previousLock = nil
	}
	var matchFound bool
	var closestValues [2]BinaryTreeValue
	node, matchFound = btpStep(node, func(node *BinaryTreeParallel, value BinaryTreeValue) int {
		comparisonResult := value.CompareTo(node.value)
		if comparisonResult != 0 {
			sideToDeleteFrom := 0
			if comparisonResult > 0 {
				sideToDeleteFrom = 1
			}
			// adjust weights
			btpAdjustPossibleWtAdj(node, 1-sideToDeleteFrom, 1, logFunction, &semaphoreLockCount)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				prevAdjustWeights()
				if matchFound {
					btpAdjustWeightAndPossibleWtAdj(node, 0-comparisonResult, logFunction, &semaphoreLockCount)
				} else {
					btpAdjustPossibleWtAdj(node, 1-sideToDeleteFrom, -1, logFunction, &semaphoreLockCount)
				}
				logFunction(func() string { return fmt.Sprintf("adjusted weights %s", btpGetNodeString(node)) })
				btpRebalanceIfNecessary(node, onRebalance, logFunction, processCounterChannel, &semaphoreLockCount)
			}
			// check if branchBounds need to be adjusted
			if value.CompareTo(node.branchBoundaries[sideToDeleteFrom]) == 0 {
				mustMatch = true
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					prevAdjustChildBounds()
					node.branchBoundaries[sideToDeleteFrom] = closestValues[1-sideToDeleteFrom]
					node.branchBMutices[sideToDeleteFrom].Unlock(logFunction, &semaphoreLockCount)
					logFunction(func() string { return fmt.Sprintf("adjusted boundaries %s", btpGetNodeString(node)) })
				}
			} else {
				node.branchBMutices[sideToDeleteFrom].Unlock(logFunction, &semaphoreLockCount)
			}
			// adjust closestValues
			closestValues[1-sideToDeleteFrom] = node.value
			node.leftright[sideToDeleteFrom].branchBMutices[0].Lock(logFunction, &semaphoreLockCount)
			node.leftright[sideToDeleteFrom].branchBMutices[1].Lock(logFunction, &semaphoreLockCount)
			node.branchBMutices[1-sideToDeleteFrom].Unlock(logFunction, &semaphoreLockCount)
		}
		return comparisonResult
	}, value, logFunction, &semaphoreLockCount)
	if matchFound {
		node.leftright[0].mutex.Lock(logFunction, &semaphoreLockCount)
		node.leftright[1].mutex.Lock(logFunction, &semaphoreLockCount)
		// adjust closest values
		if node.leftright[0].value != nil {
			closestValues[0] = btpGetBranchBoundary(node.leftright[0], 1, logFunction, &semaphoreLockCount)
		}
		if node.leftright[1].value != nil {
			closestValues[1] = btpGetBranchBoundary(node.leftright[1], 0, logFunction, &semaphoreLockCount)
		}
		// remove it
		if node.leftright[0].value == nil && node.leftright[1].value == nil {
			node.leftright[0].mutex.Unlock(logFunction, &semaphoreLockCount)
			node.leftright[1].mutex.Unlock(logFunction, &semaphoreLockCount)
			node.value = nil
			node.leftright[0] = nil
			node.leftright[1] = nil
			node.branchBoundaries[0] = nil
			node.branchBoundaries[1] = nil
			logFunction(func() string { return fmt.Sprintf("deleted leaf %s", btpGetNodeString(node)) })
		} else {
			sideToDeleteFrom := 0
			node.weightMutex.Lock(logFunction, &semaphoreLockCount)
			if node.currentWeight > 0 || node.currentWeight == 0 && node.possibleWtAdjust[1] > 0 {
				node.currentWeight--
				sideToDeleteFrom = 1
			} else {
				node.currentWeight++
			}
			node.weightMutex.Unlock(logFunction, &semaphoreLockCount)

			// update with new value
			node.value = btpGetBranchBoundary(node.leftright[sideToDeleteFrom], 1-sideToDeleteFrom, logFunction, &semaphoreLockCount)
			if node.value == nil {
				logFunction(func() string {
					return "Delete should not replace a deleted value that has one or more branches with a nil value"
				})
				panic("Delete should not replace a deleted value that has one or more branches with a nil value")
			}
			// update branch boundary if old value was one of them
			if node.leftright[1-sideToDeleteFrom].value == nil {
				node.branchBoundaries[1-sideToDeleteFrom] = node.value
			}
			var deleteWaitMutex sync.Mutex
			// delete new value from old location, and wait until deletion starts before exiting
			deleteWaitMutex.Lock()
			processCounterChannel <- 1
			go func() {
				defer func() { processCounterChannel <- -1 }()
				BTPDelete(node.leftright[sideToDeleteFrom], node.value, &deleteWaitMutex, onRebalance, true, logFunction, processCounterChannel)
			}()
			node.leftright[0].mutex.Unlock(logFunction, &semaphoreLockCount)
			node.leftright[1].mutex.Unlock(logFunction, &semaphoreLockCount)
			deleteWaitMutex.Lock()
			logFunction(func() string { return fmt.Sprintf("deleted branching node %s", btpGetNodeString(node)) })
		}
	} else if mustMatch {
		logFunction(func() string { return "Failed to delete when value was known to exist" })
		panic("Failed to delete when value was known to exist")
	} else {
		logFunction(func() string { return "node to delete not found" })
	}

}

// btpRebalance Can run in it's own goroutine and will not have a guaranteed start time in regards to parallel searches, inserts and deletions
func btpRebalance(node *BinaryTreeParallel, onRebalance func(), logFunction btpLogFunction, processCounterChannel chan int) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			logFunction(func() string { return "btpRebalance did not release all of it's locks" })
			panic("btpRebalance did not release all of it's locks")
		}
	}()
	if node == nil || node.value == nil {
		logFunction(func() string { return "btpRebalance called on a nil value" })
		panic("btpRebalance called on a nil value")
	}
	logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPRebalance", node.value.ToString()))
	node.mutex.Lock(logFunction, &semaphoreLockCount)
	defer func() {
		node.rebalancing = false
		btpRebalanceIfNecessary(node, onRebalance, logFunction, processCounterChannel, &semaphoreLockCount)
		node.mutex.Unlock(logFunction, &semaphoreLockCount)
	}()
	node.weightMutex.Lock(logFunction, &semaphoreLockCount)
	defer node.weightMutex.Unlock(logFunction, &semaphoreLockCount)
	if node.currentWeight+node.possibleWtAdjust[1] > -2 && node.currentWeight-node.possibleWtAdjust[0] < 2 {
		return
	}
	if onRebalance != nil {
		onRebalance()
	}
	newRootSide := 0            // side from which the new root node is being taken from
	if node.currentWeight > 0 { // swap sides if the weight is positive
		newRootSide = 1
	}
	rootNewSide := 1 - newRootSide // side where the old root node is being moved to

	// get the new root value
	newRootValue := btpGetBranchBoundary(node.leftright[newRootSide], rootNewSide, logFunction, &semaphoreLockCount)
	if newRootValue == nil {
		logFunction(func() string { return "BTPRebalance should not replace root's value with a nil value" })
		panic("BTPRebalance should not replace root's value with a nil value")
	}

	// adjust the binaryTree
	valueToInsert := node.value
	node.value = newRootValue
	node.currentWeight += 4*rootNewSide - 2

	var deleteStartedMutex, insertStartedMutex sync.Mutex
	// insert the oldRootValue on the rootNewSide
	insertStartedMutex.Lock()
	processCounterChannel <- 1
	go func() {
		defer func() { processCounterChannel <- -1 }()
		BTPInsert(node.leftright[rootNewSide], valueToInsert, &insertStartedMutex, onRebalance, logFunction, processCounterChannel)
	}()

	// delete the newRootValue from the newRootSide
	deleteStartedMutex.Lock()
	processCounterChannel <- 1
	go func() {
		defer func() { processCounterChannel <- -1 }()
		BTPDelete(node.leftright[newRootSide], newRootValue, &deleteStartedMutex, onRebalance, true, logFunction, processCounterChannel)
	}()

	// wait for the insert and delete to have started before continuing
	deleteStartedMutex.Lock()
	insertStartedMutex.Lock()
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func btpGetValue(binaryTree *BinaryTreeParallel) string {
	if binaryTree != nil && binaryTree.value != nil {
		return binaryTree.value.ToString()
	}
	return "nil"
}

// BTPGetNext returns the next BinaryTreeParrallel object, returns nil when it reaches the end
// not safe while values are being concurrently inserted
func BTPGetNext(binaryTree *BinaryTreeParallel) *BinaryTreeParallel {
	if binaryTree == nil || binaryTree.value == nil {
		return nil
	}
	if binaryTree.leftright[1].value != nil {
		binaryTree = binaryTree.leftright[1]
		for binaryTree.leftright[0].value != nil {
			binaryTree = binaryTree.leftright[0]
		}
		return binaryTree
	}
	for binaryTree.parent != nil && binaryTree.parent.leftright[1].value == binaryTree.value {
		binaryTree = binaryTree.parent
	}
	return binaryTree.parent
}

// BTPGetFirst returns the first value in the tree, or nil if the tree contains no values
// not safe while values are being concurrently inserted
func BTPGetFirst(binaryTree *BinaryTreeParallel) *BinaryTreeParallel {
	if binaryTree == nil || binaryTree.value == nil {
		return nil
	}
	for binaryTree.parent != nil {
		binaryTree = binaryTree.parent
	}
	for binaryTree.leftright[0].value != nil {
		binaryTree = binaryTree.leftright[0]
	}
	return binaryTree
}
