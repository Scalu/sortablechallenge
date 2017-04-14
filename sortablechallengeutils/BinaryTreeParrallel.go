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
	childBounds      [2]interface{}
	childBoundsMutex sync.Mutex
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
	if binaryTree == nil {
		return // defaults to nil, false
	}
	for binaryTree.node != nil && !matchFound {
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
		binaryTree = nextStep
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
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value interface{}) int {
		return compareValue(binaryTree.node.value, value)
	}, value, logID)
	if matchFound {
		if binaryTree.node.deleted {
			return nil
		}
		return binaryTree
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
			node.childBoundsMutex.Lock()
			if node.childBounds[0] == nil || compareValue(node.childBounds[0], value) == -1 {
				node.childBounds[0] = value
			}
			if node.childBounds[1] == nil || compareValue(node.childBounds[0], value) == 1 {
				node.childBounds[1] = value
			}
			node.childBoundsMutex.Unlock()
		}
		if node.weight+node.possibleWtAdj[1] < -1 || node.weight-node.possibleWtAdj[0] > 1 {
			go btpRebalance(binaryTree, compareValue, onRebalance, logID)
		}
		return
	}, value, logID)
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
	adjustChildBounds := func() {} // does nothing for now
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
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value interface{}) (comparisonResult int) {
		node := binaryTree.node
		comparisonResult = compareValue(node.value, value)
		if comparisonResult != 0 {
			node.childBoundsMutex.Lock()
			if compareValue(node.childBounds[0], value) == 0 || compareValue(node.childBounds[1], value) == 0 {
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					//need to figure this out
					ack
					node.childBoundsMutex.Unlock()
					prevAdjustChildBounds()
				}
			} else {
				node.childBoundsMutex.Unlock()
			}
		}
		return
	}, value, logID)
	if matchFound {
		// remove it
		ack
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
	rootWeightMod := 1 // weight modifier for nodes between the current and new roots
	rootNewSide := 1   // side where the old root node is being moved to
	if node.weight > 0 { // swap sides if the weight is positive
		newRootSide = 1
		rootWeightMod = -1
		rootNewSide = 0
	}

	// get the new root value
	newRoot := node.leftright[newRootSide]
	btpLock(newRoot, logID)
	semaphoreLockCount++
	newRootValue := newRoot.node.value
	newRoot.node.childBoundsMutex.Lock()
	if newRoot.node.childBounds[rootNewSide] != nil && compareValue(newRoot.node.childBounds[rootNewSide], newRootValue) == rootWeightMode {
		newRootValue = newRoot.node.childBounds[rootNewSide]
	}
	newRoot.node.childBoundsMutex.Unlock()
	btpUnlock(newRoot, logID)
	semaphoreLockCount--

	var deleteStartedMutex, insertStartedMutex sync.Mutex
	// delete the newRootValue from the newRootSide
	deleteStartedMutex.Lock()
	go BTPDelete(node.leftright[newRootSide], compareValue, newRootValue, &deleteStartedMutex, onRebalance, logID)

	// insert the oldRootValue on the rootNewSide
	insertStartedMutex.Lock()
	go BTPInsert(node.leftright[rootNewSide], binaryTree, compareValue, node.value, &insertStartedMutex, onRebalance, logID)

	// set the value on the binaryTree and adjust the weight
	node.value = newRootValue
	node.weight += 2*rootWeightMod
	ack // need to adjust child bounds
	
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
