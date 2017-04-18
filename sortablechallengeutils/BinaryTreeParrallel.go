package sortablechallengeutils

import (
	"fmt"
	"sync"
)

// BinaryTreeValue is the interface for values being stored into the binary tree
type BinaryTreeValue interface {
	CompareTo(BinaryTreeValue) int
	ToString() string
}

type btpMutex struct {
	mutexID     string
	mutex       sync.Mutex
	lockCounter int
	currentLock int
	node        *BinaryTreeParallel
}

func (mutex *btpMutex) Lock(logFunction btpLogFunction) {
	if mutex.currentLock > 0 {
		logFunction(func() string {
			return fmt.Sprintf("mutex %s already locked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node))
		})
	}
	mutex.mutex.Lock()
	mutex.lockCounter++
	mutex.currentLock = mutex.lockCounter
	logFunction(func() string {
		return fmt.Sprintf("mutex %s locked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node))
	})
}

func (mutex *btpMutex) Unlock(logFunction btpLogFunction) {
	logFunction(func() string {
		return fmt.Sprintf("mutex %s unlocked, count %d, node %s", mutex.mutexID, mutex.currentLock, btpGetNodeString(mutex.node))
	})
	mutex.currentLock = 0
	mutex.mutex.Unlock()
}

// BinaryTreeParallel creates a binary tree that supports concurrent inserts, deletes and searches
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

type btpLogFunction func(func() string) bool

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

func btpGetWeight(node *BinaryTreeParallel, logFunction btpLogFunction) int {
	node.weightMutex.Lock(logFunction)
	defer node.weightMutex.Unlock(logFunction)
	return node.currentWeight
}

func btpAdjustPossibleWtAjd(node *BinaryTreeParallel, side int, amount int, logFunction btpLogFunction) {
	node.weightMutex.Lock(logFunction)
	defer node.weightMutex.Unlock(logFunction)
	node.possibleWtAdjust[side] += amount
}

func btpAdjustWeightAndPossibleWtAdj(node *BinaryTreeParallel, amount int, logFunction btpLogFunction) {
	node.weightMutex.Lock(logFunction)
	defer node.weightMutex.Unlock(logFunction)
	node.currentWeight += amount
	if amount > 0 {
		node.possibleWtAdjust[1] -= amount
	} else {
		node.possibleWtAdjust[0] += amount
	}
}

func btpRebalanceIfNecessary(binaryTree *BinaryTreeParallel, onRebalance func(), logFunction btpLogFunction, processCounterChannel chan int) {
	binaryTree.weightMutex.Lock(logFunction)
	defer binaryTree.weightMutex.Unlock(logFunction)
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

func btpGetBranchBoundary(node *BinaryTreeParallel, side int, logFunction btpLogFunction) BinaryTreeValue {
	node.branchBMutices[side].Lock(logFunction)
	defer node.branchBMutices[side].Unlock(logFunction)
	return node.branchBoundaries[side]
}

func btpAdjustChildBounds(node *BinaryTreeParallel, value BinaryTreeValue, logFunction btpLogFunction) {
	if value == nil {
		return
	}
	comparisonResult := value.CompareTo(node.value)
	if comparisonResult == -1 {
		node.branchBMutices[0].Lock(logFunction)
		if value.CompareTo(node.branchBoundaries[0]) == -1 {
			node.branchBoundaries[0] = value
		}
		node.branchBMutices[0].Unlock(logFunction)
	} else {
		node.branchBMutices[1].Lock(logFunction)
		if value.CompareTo(node.branchBoundaries[1]) == 1 {
			node.branchBoundaries[1] = value
		}
		node.branchBMutices[1].Unlock(logFunction)
	}
}

func btpStep(binaryTree *BinaryTreeParallel, compareValues func(*BinaryTreeParallel, BinaryTreeValue) int, value BinaryTreeValue, logFunction btpLogFunction) (nextStep *BinaryTreeParallel, matchFound bool) {
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
			nextStep.mutex.Lock(logFunction)
			binaryTree.mutex.Unlock(logFunction)
		}
		binaryTree = nextStep
	}
	return
}

// BTPSearch call with a go routing for concurrency
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
	binaryTree.mutex.Lock(logFunction) // lock the current tree
	semaphoreLockCount++
	defer func() {
		binaryTree.mutex.Unlock(logFunction)
		semaphoreLockCount--
	}()
	if previousLock != nil {
		previousLock.Unlock()
	}
	var matchFound bool
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value BinaryTreeValue) int {
		return value.CompareTo(binaryTree.value)
	}, value, logFunction)
	if matchFound {
		return binaryTree.value
	}
	return nil
}

// BTPInsert call with a go routine for concurrency
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
	binaryTree.mutex.Lock(logFunction)
	semaphoreLockCount++
	defer func() {
		binaryTree.mutex.Unlock(logFunction)
		semaphoreLockCount--
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
			btpAdjustPossibleWtAjd(binaryTree, sideIndex, 1, logFunction)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				if !matchFound {
					btpAdjustWeightAndPossibleWtAdj(binaryTree, comparisonResult, logFunction)
				} else {
					btpAdjustPossibleWtAjd(binaryTree, sideIndex, -1, logFunction)
				}
				logFunction(func() string { return fmt.Sprintf("adjusting weights %s", btpGetNodeString(binaryTree)) })
				btpRebalanceIfNecessary(binaryTree, onRebalance, logFunction, processCounterChannel)
				prevAdjustWeights()
			}
			btpAdjustChildBounds(binaryTree, value, logFunction)
		}
		return
	}, value, logFunction)
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

// BTPDelete call with a go routine for concurrency
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
	node.mutex.Lock(logFunction)
	semaphoreLockCount++
	defer func() {
		node.mutex.Unlock(logFunction)
		semaphoreLockCount--
	}()
	if previousLock != nil {
		previousLock.Unlock()
		previousLock = nil
	}
	var matchFound bool
	var closestValues [2]BinaryTreeValue
	node, matchFound = btpStep(node, func(node *BinaryTreeParallel, value BinaryTreeValue) (comparisonResult int) {
		comparisonResult = value.CompareTo(node.value)
		if comparisonResult != 0 {
			sideToDeleteFrom := 0
			if comparisonResult > 0 {
				sideToDeleteFrom = 1
			}
			// adjust weights
			btpAdjustPossibleWtAjd(node, 1-sideToDeleteFrom, 1, logFunction)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				if matchFound {
					btpAdjustWeightAndPossibleWtAdj(node, 0-comparisonResult, logFunction)
				} else {
					btpAdjustPossibleWtAjd(node, 1-sideToDeleteFrom, -1, logFunction)
				}
				logFunction(func() string { return fmt.Sprintf("adjusting weights %s", btpGetNodeString(node)) })
				btpRebalanceIfNecessary(node, onRebalance, logFunction, processCounterChannel)
				prevAdjustWeights()
			}
			// check if branchBounds need to be adjusted
			node.branchBMutices[sideToDeleteFrom].Lock(logFunction)
			if value.CompareTo(node.branchBoundaries[sideToDeleteFrom]) == 0 {
				mustMatch = true
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					node.branchBoundaries[sideToDeleteFrom] = closestValues[1-sideToDeleteFrom]
					node.branchBMutices[sideToDeleteFrom].Unlock(logFunction)
					logFunction(func() string { return fmt.Sprintf("adjusting boundaries %s", btpGetNodeString(node)) })
					prevAdjustChildBounds()
				}
			} else {
				node.branchBMutices[sideToDeleteFrom].Unlock(logFunction)
			}
			// adjust closestValues
			closestValues[1-sideToDeleteFrom] = node.value
		}
		return
	}, value, logFunction)
	if matchFound {
		// adjust closest values
		if node.leftright[0].value != nil {
			closestValues[0] = btpGetBranchBoundary(node.leftright[0], 1, logFunction)
		}
		if node.leftright[1].value != nil {
			closestValues[1] = btpGetBranchBoundary(node.leftright[1], 0, logFunction)
		}
		// remove it
		if node.leftright[0].value == nil && node.leftright[1].value == nil {
			node.value = nil
			node.leftright[0] = nil
			node.leftright[1] = nil
			node.branchBoundaries[0] = nil
			node.branchBoundaries[1] = nil
			logFunction(func() string { return fmt.Sprintf("deleted leaf %s", btpGetNodeString(node)) })
		} else {
			sideToDeleteFrom := 0
			node.weightMutex.Lock(logFunction)
			if node.currentWeight > 0 {
				node.currentWeight--
				sideToDeleteFrom = 1
			} else {
				node.currentWeight++
			}
			node.weightMutex.Unlock(logFunction)

			// update with new value
			node.value = btpGetBranchBoundary(node.leftright[sideToDeleteFrom], 1-sideToDeleteFrom, logFunction)
			// update branch boundary if old value is one of them
			if node.leftright[1-sideToDeleteFrom].value == nil {
				node.branchBMutices[1-sideToDeleteFrom].Lock(logFunction)
				node.branchBoundaries[1-sideToDeleteFrom] = node.value
				node.branchBMutices[1-sideToDeleteFrom].Unlock(logFunction)
			}
			var deleteWaitMutex sync.Mutex
			// delete new value from old location, and wait until deletion starts before exiting
			deleteWaitMutex.Lock()
			processCounterChannel <- 1
			go func() {
				defer func() { processCounterChannel <- -1 }()
				BTPDelete(node.leftright[sideToDeleteFrom], node.value, &deleteWaitMutex, onRebalance, true, logFunction, processCounterChannel)
			}()
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

// btpRebalance call with a goroutine
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
	node.mutex.Lock(logFunction)
	semaphoreLockCount++
	defer func() {
		node.rebalancing = false
		btpRebalanceIfNecessary(node, onRebalance, logFunction, processCounterChannel)
		node.mutex.Unlock(logFunction)
		semaphoreLockCount--
	}()
	node.weightMutex.Lock(logFunction)
	defer node.weightMutex.Unlock(logFunction)
	if node.currentWeight+node.possibleWtAdjust[1] > -2 && node.currentWeight-node.possibleWtAdjust[0] < 2 {
		return
	}
	if onRebalance != nil {
		onRebalance()
	}
	newRootSide := 0            // side from which the new root node is being taken from
	rootWeightMod := 1          // weight modifier for nodes between the current and new roots
	rootNewSide := 1            // side where the old root node is being moved to
	if node.currentWeight > 0 { // swap sides if the weight is positive
		newRootSide = 1
		rootWeightMod = -1
		rootNewSide = 0
	}

	// get the new root value
	newRoot := node.leftright[newRootSide]
	newRootValue := btpGetBranchBoundary(newRoot, rootNewSide, logFunction)

	var deleteStartedMutex, insertStartedMutex sync.Mutex
	// delete the newRootValue from the newRootSide
	deleteStartedMutex.Lock()
	processCounterChannel <- 1
	go func() {
		defer func() { processCounterChannel <- -1 }()
		BTPDelete(node.leftright[newRootSide], newRootValue, &deleteStartedMutex, onRebalance, true, logFunction, processCounterChannel)
	}()
	// insert the oldRootValue on the rootNewSide
	insertStartedMutex.Lock()
	processCounterChannel <- 1
	go func() {
		defer func() { processCounterChannel <- -1 }()
		BTPInsert(node.leftright[rootNewSide], node.value, &insertStartedMutex, onRebalance, logFunction, processCounterChannel)
	}()
	// wait for the insert and delete to have started before continuing
	deleteStartedMutex.Lock()
	insertStartedMutex.Lock()

	// adjust the binaryTree
	node.value = newRootValue
	node.currentWeight += 2 * rootWeightMod
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
