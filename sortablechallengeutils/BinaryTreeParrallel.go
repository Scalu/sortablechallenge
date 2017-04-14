package sortablechallengeutils

import (
	"fmt"
	"sync"
)

type binaryTreeParallelNode struct {
	value            interface{}
	weight           int
	possibleWtAdj    [2]int // possible weight adjustments pending inserts and deletions
	parent           *BinaryTreeParallel
	leftright        [2]*BinaryTreeParallel
	rebalancing      bool
	deleted          bool
	rebalanceValue   int // -1 if weight was negative, +1 if it was positive
	closestInsert    interface{}
	closestIsDeleted bool
}

// BinaryTreeParallel creates a binary tree that supports concurrent inserts and searches
// use BTPGetNew() to properly create one
type BinaryTreeParallel struct {
	mutex sync.Mutex
	node  *binaryTreeParallelNode
}

func btpLock(binaryTree *BinaryTreeParallel, logID *string) {
	var parent, left, right *BinaryTreeParallel
	var weight int
	if binaryTree.node != nil {
		parent = binaryTree.node.parent
		left = binaryTree.node.leftright[0]
		right = binaryTree.node.leftright[1]
		weight = binaryTree.node.weight
	}
	fmt.Printf("%s Locking   btp %d, parent %d, left %d, right %d, weight %d\n", *logID, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
	binaryTree.mutex.Lock()
	fmt.Printf("%s Locked    btp %d, parent %d, left %d, right %d, weight %d\n", *logID, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
}

func btpUnlock(binaryTree *BinaryTreeParallel, logID *string) {
	var parent, left, right *BinaryTreeParallel
	var weight int
	if binaryTree.node != nil {
		parent = binaryTree.node.parent
		left = binaryTree.node.leftright[0]
		right = binaryTree.node.leftright[1]
		weight = binaryTree.node.weight
	}
	fmt.Printf("%s Unlocking btp %d, parent %d, left %d, right %d, weight %d\n", *logID, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
	binaryTree.mutex.Unlock()
	fmt.Printf("%s Unlocked  btp %d, parent %d, left %d, right %d, weight %d\n", *logID, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
}

func btpStep(binaryTree *BinaryTreeParallel, compareValues func(*BinaryTreeParallel, interface{}) int, value interface{}, logID *string) (nextStep *BinaryTreeParallel, matchFound bool) {
	if binaryTree.node == nil {
		return // defaults to nil, false
	}
	comparisonResult := compareValues(binaryTree, value)
	switch comparisonResult {
	case -1:
		nextStep = binaryTree.node.leftright[0]
	case 1:
		nextStep = binaryTree.node.leftright[1]
	case 0:
		nextStep = binaryTree
		matchFound = true
	default:
		panic(fmt.Sprintf("%s compareValues function returned invalid comparison value %d", *logID, comparisonResult))
	}
	if comparisonResult != 0 {
		btpLock(nextStep, logID)
		btpUnlock(binaryTree, logID)
	}
	return
}

// BTPSearch call with a go routing for concurrency
func BTPSearch(binaryTree *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, value interface{}, previousLock *sync.Mutex, logID *string) *BinaryTreeParallel {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s BTPSearch did not release all of it's locks", *logID))
		}
	}()
	if binaryTree == nil {
		panic(fmt.Sprintf("%s BTPSearch should not be called with a nil binaryTree value", *logID))
	}
	btpLock(binaryTree, logID) // lock the current tree
	semaphoreLockCount++
	defer func() {
		btpUnlock(binaryTree, logID)
		semaphoreLockCount--
	}()
	if previousLock != nil {
		previousLock.Unlock()
	}
	var matchFound bool
	for binaryTree.node != nil {
		binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value interface{}) int {
			return compareValue(binaryTree.node.value, value)
		}, value, logID)
		if matchFound {
			if binaryTree.node.deleted {
				return nil
			}
			return binaryTree
		}
	}
	return nil
}

// BTPInsert call with a go routine for concurrency
func BTPInsert(binaryTree *BinaryTreeParallel, parent *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, value interface{}, previousLock *sync.Mutex, onRebalance func(), logID *string) *BinaryTreeParallel {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s BTPInsert did not release all of it's locks", *logID))
		}
	}()
	if binaryTree == nil {
		panic(fmt.Sprintf("%s BTPInsert should not be called with a nil binaryTree value", *logID))
	}
	adjustWeights := func() {} // does nothing yet
	defer func() { adjustWeights() }()
	btpLock(binaryTree, logID)
	semaphoreLockCount++
	defer func() {
		btpUnlock(binaryTree, logID)
		semaphoreLockCount--
	}()
	if previousLock != nil {
		previousLock.Unlock()
		previousLock = nil
	}
	var matchFound bool
	for binaryTree.node != nil && !matchFound {
		parent = binaryTree
		binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value interface{}) (comparisonResult int) {
			node := binaryTree.node
			comparisonResult = compareValue(node.value, value)
			if comparisonResult != 0 {
				sideIndex := 0
				if comparisonResult > 0 {
					sideIndex = 1
				}
				node.possibleWtAdj[sideIndex]++
				prevAdjustWeights := adjustWeights
				adjustWeights = func() {
					btpLock(binaryTree, logID)
					node.possibleWtAdj[sideIndex]--
					if matchFound {
						node.weight += comparisonResult
					}
					if !node.rebalancing && (node.weight+node.possibleWtAdj[1] < -1 || node.weight-node.possibleWtAdj[0] > 1) {
						node.rebalancing = true
						go btpRebalance(binaryTree, compareValue, onRebalance, logID)
					}
					btpUnlock(binaryTree, logID)
					prevAdjustWeights()
				}
			}
			if node.rebalancing {
				if node.rebalanceValue == comparisonResult && (node.closestInsert == nil || compareValue(value, node.closestInsert) == node.rebalanceValue) {
					node.closestInsert = value
					node.closestIsDeleted = false
				}
			} else if node.weight+node.possibleWtAdj[1] < -1 || node.weight-node.possibleWtAdj[0] > 1 {
				node.rebalancing = true
				go btpRebalance(binaryTree, compareValue, onRebalance, logID)
			}
			return
		}, value, logID)
	}
	if matchFound && binaryTree.node.deleted == true {
		binaryTree.node.deleted = false
	}
	if binaryTree.node == nil {
		binaryTree.node = &binaryTreeParallelNode{value: value, parent: parent}
		binaryTree.node.leftright[0] = &BinaryTreeParallel{}
		binaryTree.node.leftright[1] = &BinaryTreeParallel{}
	}
	return binaryTree
}

// BTPDelete call with a go routine for concurrency
func BTPDelete(binaryTree *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, value interface{}, previousLock *sync.Mutex, onRebalance func(), logID *string) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s BTPDelete did not release all of it's locks", *logID))
		}
	}()
	if binaryTree == nil {
		panic(fmt.Sprintf("%s BTPDelete should not be called with a nil binaryTree value", *logID))
	}
	btpLock(binaryTree, logID)
	semaphoreLockCount++
	defer func() {
		btpUnlock(binaryTree, logID)
		semaphoreLockCount--
	}()
	if previousLock != nil {
		previousLock.Unlock()
		previousLock = nil
	}
	var matchFound bool
	for binaryTree.node != nil && !matchFound {
		binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value interface{}) (comparisonResult int) {
			comparisonResult = compareValue(binaryTree.node.value, value)
			if binaryTree.node.rebalancing && compareValue(binaryTree.node.closestInsert, value) == 0 {
				binaryTree.node.closestIsDeleted = true
			}
			return
		}, value, logID)
	}
	if matchFound {
		// remove it
		binaryTree.node.deleted = true
	}
}

// btpRebalance call with a goroutine
func btpRebalance(binaryTree *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, onRebalance func(), logID *string) {
	rebalanceLogID := fmt.Sprintf("%s rebalance", *logID)
	logID = &rebalanceLogID
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s btpRebalance did not release all of it's locks", *logID))
		}
	}()
	btpLock(binaryTree, logID)
	semaphoreLockCount++
	node := binaryTree.node
	defer func() {
		if node != nil && (node.weight+node.possibleWtAdj[1] < -1 || node.weight-node.possibleWtAdj[0] > 1) {
			node.rebalancing = true
			go btpRebalance(binaryTree, compareValue, onRebalance, logID)
		}
		btpUnlock(binaryTree, logID)
		semaphoreLockCount--
	}()
	if node == nil {
		panic(fmt.Sprintf("%s btpRebalance called on a nil value", *logID))
	}
	if node.weight+node.possibleWtAdj[1] > -2 && node.weight-node.possibleWtAdj[0] < 2 {
		return
	}
	if onRebalance != nil {
		onRebalance()
	}
	newRootSide := 0   // side from which the new root node is being taken from
	rootWeightMod := 2 // weight modifier for nodes between the current and new roots
	rootNewSide := 1   // side where the old root node is being moved to
	node.rebalanceValue = -1
	if node.weight > 0 { // swap sides if the weight is positive
		newRootSide = 1
		rootWeightMod = -2
		rootNewSide = 0
		node.rebalanceValue = 1
	}

	// get the new root value
	newRoot := node.leftright[newRootSide]
	btpLock(newRoot, logID)
	semaphoreLockCount++
	btpUnlock(binaryTree, logID)
	semaphoreLockCount--
	for newRoot.node.leftright[rootNewSide].node != nil {
		btpLock(newRoot.node.leftright[rootNewSide], logID) // lock the next newRoot candidate
		semaphoreLockCount++
		btpUnlock(newRoot, logID) // unlock the old newRoot
		semaphoreLockCount--
		newRoot = newRoot.node.leftright[rootNewSide] // assign the new newRoot
	}
	newRootValue := newRoot.node.value
	newRootValueDeleted := newRoot.node.deleted
	btpUnlock(newRoot, logID)
	semaphoreLockCount--

	// relock the binaryTree
	btpLock(binaryTree, logID)
	semaphoreLockCount++
	//get the node ready to be balanced
	node.rebalancing = false
	if compareValue(newRootValue, node.value) != compareValue(node.closestInsert, newRootValue) {
		newRootValue = node.closestInsert
		newRootValueDeleted = node.closestIsDeleted
	}
	node.closestInsert = nil
	// check if new inserts have rebalanced the tree
	if (newRootSide == 0 && node.weight+node.possibleWtAdj[1] > -2) || (newRootSide == 1 && node.weight-node.possibleWtAdj[0] < 2) {
		return
	}

	var deleteStartedMutex, insertStartedMutex sync.Mutex
	// delete the newRootValue from the newRootSide
	if !newRootValueDeleted {
		deleteStartedMutex.Lock()
		go BTPDelete(node.leftright[newRootSide], compareValue, newRootValue, &deleteStartedMutex, onRebalance, logID)
	}

	// insert the oldRootValue on the rootNewSide
	if node.deleted == false {
		insertStartedMutex.Lock()
		go BTPInsert(node.leftright[rootNewSide], binaryTree, compareValue, node.value, &insertStartedMutex, onRebalance, logID)
	}

	// set the value on the binaryTree and adjust the weight
	node.value = newRootValue
	node.deleted = newRootValueDeleted
	node.weight += rootWeightMod
	// wait for the insert and delete to have started before exiting (and releasing the lock on the root)
	deleteStartedMutex.Lock()
	insertStartedMutex.Lock()
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func BTPGetValue(binaryTree *BinaryTreeParallel) interface{} {
	if binaryTree != nil && binaryTree.node != nil {
		return binaryTree.node.value
	}
	return -1
}

// BTPGetNext returns the next BinaryTreeParrallel object, returns nil when it reaches the end
// not safe while values are being concurrently inserted
func BTPGetNext(binaryTree *BinaryTreeParallel) *BinaryTreeParallel {
	if binaryTree == nil || binaryTree.node == nil {
		return nil
	}
	if binaryTree.node.leftright[1].node != nil {
		binaryTree = binaryTree.node.leftright[1]
		for binaryTree.node.leftright[0].node != nil {
			binaryTree = binaryTree.node.leftright[0]
		}
		return binaryTree
	}
	for binaryTree.node.parent != nil && binaryTree.node.parent.node.leftright[1].node != nil && binaryTree.node.parent.node.leftright[1].node.value == binaryTree.node.value {
		binaryTree = binaryTree.node.parent
	}
	return binaryTree.node.parent
}

// BTPGetFirst returns the first value in the tree, or nil if the tree contains no values
// not safe while values are being concurrently inserted
func BTPGetFirst(binaryTree *BinaryTreeParallel) *BinaryTreeParallel {
	if binaryTree == nil || binaryTree.node == nil {
		return nil
	}
	for binaryTree.node.parent != nil {
		binaryTree = binaryTree.node.parent
	}
	for binaryTree.node.leftright[0].node != nil {
		binaryTree = binaryTree.node.leftright[0]
	}
	return binaryTree
}
