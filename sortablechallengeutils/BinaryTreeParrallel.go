package sortablechallengeutils

import (
	"fmt"
	"sync"
)

// BinaryTreeParallel creates a binary tree that supports concurrent inserts, deletes and searches
type BinaryTreeParallel struct {
	mutex            sync.Mutex
	value            interface{}
	weight           int
	possibleWtAdj    [2]int // possible weight adjustments pending inserts and deletions
	parent           *BinaryTreeParallel
	leftright        [2]*BinaryTreeParallel
	branchBoundaries [2]interface{}
	branchBMutices   [2]sync.Mutex
	rebalancing      bool
}

func btpGetNodeString(node *BinaryTreeParallel) string {
	if node == nil {
		return "nil node"
	}
	return fmt.Sprintf("btp %d, parent %d, left %d, right %d, branch bounds %d - %d, weight %d, possible weight mods -%d +%d",
		BTPGetValue(node), BTPGetValue(node.parent), BTPGetValue(node.leftright[0]), BTPGetValue(node.leftright[1]),
		node.branchBoundaries[0].(int), node.branchBoundaries[1].(int), node.weight, node.possibleWtAdj[0], node.possibleWtAdj[1])
}

func btpLock(binaryTree *BinaryTreeParallel, logID *string) {
	if logID != nil {
		fmt.Printf("%s Locking   %s\n", *logID, btpGetNodeString(binaryTree))
	}
	binaryTree.mutex.Lock()
	if logID != nil {
		fmt.Printf("%s Locked    %s\n", *logID, btpGetNodeString(binaryTree))
	}
}

func btpUnlock(binaryTree *BinaryTreeParallel, logID *string) {
	if logID != nil {
		fmt.Printf("%s Unlocking %s\n", *logID, btpGetNodeString(binaryTree))
	}
	binaryTree.mutex.Unlock()
	if logID != nil {
		fmt.Printf("%s Unlocked  %s\n", *logID, btpGetNodeString(binaryTree))
	}
}

func btpGetBranchBoundary(node *BinaryTreeParallel, side int) interface{} {
	node.branchBMutices[side].Lock()
	defer node.branchBMutices[side].Unlock()
	return node.branchBoundaries[side]
}

func btpAdjustChildBounds(node *BinaryTreeParallel, value interface{}, compareValues func(interface{}, interface{}) int, logID *string) {
	if value == nil {
		return
	}
	comparisonResult := compareValues(node.value, value)
	if comparisonResult == -1 {
		if compareValues(btpGetBranchBoundary(node, 0), value) == -1 {
			node.branchBoundaries[0] = value
		}
	} else {
		if compareValues(btpGetBranchBoundary(node, 0), value) == 1 {
			node.branchBoundaries[1] = value
		}
	}
}

func btpStep(binaryTree *BinaryTreeParallel, compareValues func(*BinaryTreeParallel, interface{}) int, value interface{}, logID *string) (nextStep *BinaryTreeParallel, matchFound bool) {
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
func BTPSearch(binaryTree *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, value interface{}, previousLock *sync.Mutex, logID *string) interface{} {
	if logID != nil {
		newLogString := fmt.Sprintf("%s Search", *logID)
		logID = &newLogString
	}
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
		return compareValue(binaryTree.value, value)
	}, value, logID)
	if matchFound {
		return binaryTree.value
	}
	return nil
}

// BTPInsert call with a go routine for concurrency
func BTPInsert(binaryTree *BinaryTreeParallel, parent *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, value interface{}, previousLock *sync.Mutex, onRebalance func(), logID *string) interface{} {
	if logID != nil {
		newLogString := fmt.Sprintf("%s Insert", *logID)
		logID = &newLogString
	}
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
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value interface{}) (comparisonResult int) {
		comparisonResult = compareValue(binaryTree.value, value)
		if comparisonResult != 0 {
			parent = binaryTree
			sideIndex := 0
			if comparisonResult > 0 {
				sideIndex = 1
			}
			binaryTree.possibleWtAdj[sideIndex]++
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				btpLock(binaryTree, logID)
				binaryTree.possibleWtAdj[sideIndex]--
				if !matchFound {
					binaryTree.weight += comparisonResult
				}
				if !binaryTree.rebalancing && (binaryTree.weight+binaryTree.possibleWtAdj[1] < -1 || binaryTree.weight-binaryTree.possibleWtAdj[0] > 1) {
					binaryTree.rebalancing = true
					go btpRebalance(binaryTree, compareValue, onRebalance, logID)
				}
				btpUnlock(binaryTree, logID)
				prevAdjustWeights()
			}
			btpAdjustChildBounds(binaryTree, value, compareValue, logID)
		}
		if binaryTree.weight+binaryTree.possibleWtAdj[1] < -1 || binaryTree.weight-binaryTree.possibleWtAdj[0] > 1 {
			go btpRebalance(binaryTree, compareValue, onRebalance, logID)
		}
		return
	}, value, logID)
	if binaryTree.value == nil {
		binaryTree.value = value
		binaryTree.branchBoundaries[0] = value
		binaryTree.branchBoundaries[1] = value
		binaryTree.leftright[0] = &BinaryTreeParallel{parent: binaryTree}
		binaryTree.leftright[1] = &BinaryTreeParallel{parent: binaryTree}
	}
	return binaryTree.value
}

// BTPDelete call with a go routine for concurrency
func BTPDelete(node *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, value interface{}, previousLock *sync.Mutex, onRebalance func(), logID *string) {
	if logID != nil {
		newLogString := fmt.Sprintf("%s Delete", *logID)
		logID = &newLogString
	}
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s BTPDelete did not release all of it's locks", *logID))
		}
	}()
	if node == nil {
		panic(fmt.Sprintf("%s BTPDelete should not be called with a nil node value", *logID))
	}
	adjustWeights := func() {} // does nothing yet
	defer func() { adjustWeights() }()
	adjustChildBounds := func() {} // does nothing for now
	defer func() { adjustChildBounds() }()
	btpLock(node, logID)
	semaphoreLockCount++
	defer func() {
		btpUnlock(node, logID)
		semaphoreLockCount--
	}()
	if previousLock != nil {
		previousLock.Unlock()
		previousLock = nil
	}
	var matchFound bool
	var closestValues [2]interface{}
	node, matchFound = btpStep(node, func(binaryTree *BinaryTreeParallel, value interface{}) (comparisonResult int) {
		comparisonResult = compareValue(node.value, value)
		if comparisonResult != 0 {
			// adjust weights
			sideIndex := 0
			if comparisonResult < 0 {
				sideIndex = 1
			}
			node.possibleWtAdj[sideIndex]++
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				btpLock(binaryTree, logID)
				node.possibleWtAdj[sideIndex]--
				if matchFound {
					node.weight -= comparisonResult
				}
				if !node.rebalancing && (node.weight+node.possibleWtAdj[1] < -1 || node.weight-node.possibleWtAdj[0] > 1) {
					node.rebalancing = true
					go btpRebalance(binaryTree, compareValue, onRebalance, logID)
				}
				btpUnlock(binaryTree, logID)
				prevAdjustWeights()
			}
			// check if branchBounds need to be adjusted
			node.branchBMutices[0].Lock()
			if compareValue(node.branchBoundaries[0], value) == 0 {
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					node.branchBoundaries[0] = closestValues[1]
					node.branchBMutices[0].Unlock()
					prevAdjustChildBounds()
				}
			} else {
				node.branchBMutices[0].Unlock()
			}
			node.branchBMutices[1].Lock()
			if compareValue(node.branchBoundaries[1], value) == 0 {
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					node.branchBoundaries[1] = closestValues[0]
					node.branchBMutices[1].Unlock()
					prevAdjustChildBounds()
				}
			} else {
				node.branchBMutices[1].Unlock()
			}
			// adjust closestValues
			if comparisonResult == -1 {
				closestValues[1] = node.value
			}
			if comparisonResult == 1 {
				closestValues[0] = node.value
			}
		}
		return
	}, value, logID)
	if matchFound {
		// adjust closest values
		if node.leftright[0].value != nil {
			closestValues[0] = btpGetBranchBoundary(node.leftright[0], 1)
		}
		if node.leftright[1].value != nil {
			closestValues[1] = btpGetBranchBoundary(node.leftright[1], 0)
		}
		// remove it
		if node.leftright[0].value == nil && node.leftright[1].value == nil {
			node.value = nil
		} else {
			branchSide := 0
			treeToRunDeleteOn := node.leftright[1]
			if node.weight < 0 {
				node.weight++
				branchSide = 1
				treeToRunDeleteOn = node.leftright[0]
			} else {
				node.weight--
			}
			btpLock(treeToRunDeleteOn, logID)
			node.value = btpGetBranchBoundary(treeToRunDeleteOn, branchSide)
			var deleteWaitMutex sync.Mutex
			deleteWaitMutex.Lock()
			go BTPDelete(treeToRunDeleteOn, compareValue, node.value, &deleteWaitMutex, onRebalance, logID)
			btpUnlock(treeToRunDeleteOn, logID)
			deleteWaitMutex.Lock()
		}
	}
}

// btpRebalance call with a goroutine
func btpRebalance(node *BinaryTreeParallel, compareValue func(interface{}, interface{}) int, onRebalance func(), logID *string) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s btpRebalance did not release all of it's locks", *logID))
		}
	}()
	btpLock(node, logID)
	semaphoreLockCount++
	if node == nil || node.value == nil {
		panic(fmt.Sprintf("%s btpRebalance called on a nil value", *logID))
	}
	if logID != nil {
		rebalanceLogID := fmt.Sprintf("%s Rebalance %d", *logID, node.value.(int))
		logID = &rebalanceLogID
	}
	defer func() {
		if node.weight+node.possibleWtAdj[1] < -1 || node.weight-node.possibleWtAdj[0] > 1 {
			go btpRebalance(node, compareValue, onRebalance, logID)
		} else {
			node.rebalancing = false
		}
		btpUnlock(node, logID)
		semaphoreLockCount--
	}()
	if node.weight+node.possibleWtAdj[1] > -2 && node.weight-node.possibleWtAdj[0] < 2 {
		return
	}
	if onRebalance != nil {
		onRebalance()
	}
	newRootSide := 0     // side from which the new root node is being taken from
	rootWeightMod := 1   // weight modifier for nodes between the current and new roots
	rootNewSide := 1     // side where the old root node is being moved to
	if node.weight > 0 { // swap sides if the weight is positive
		newRootSide = 1
		rootWeightMod = -1
		rootNewSide = 0
	}

	// get the new root value
	newRoot := node.leftright[newRootSide]
	btpLock(newRoot, logID)
	semaphoreLockCount++
	newRootValue := btpGetBranchBoundary(newRoot, rootNewSide)
	btpUnlock(newRoot, logID)
	semaphoreLockCount--

	var deleteStartedMutex, insertStartedMutex sync.Mutex
	// delete the newRootValue from the newRootSide
	deleteStartedMutex.Lock()
	go BTPDelete(node.leftright[newRootSide], compareValue, newRootValue, &deleteStartedMutex, onRebalance, logID)

	// insert the oldRootValue on the rootNewSide
	insertStartedMutex.Lock()
	go BTPInsert(node.leftright[rootNewSide], node, compareValue, node.value, &insertStartedMutex, onRebalance, logID)
	btpAdjustChildBounds(node, node.value, compareValue, logID)

	// wait for the insert and delete to have started before continuing
	deleteStartedMutex.Lock()
	insertStartedMutex.Lock()

	// adjust the binaryTree
	node.value = newRootValue
	node.weight += 2 * rootWeightMod
	node.rebalancing = false
	node.branchBMutices[newRootSide].Lock()
	node.branchBoundaries[newRootSide] = node.value
	node.branchBMutices[newRootSide].Unlock()
	btpLock(newRoot, logID)
	semaphoreLockCount++
	if newRoot.value != nil {
		btpAdjustChildBounds(node, btpGetBranchBoundary(newRoot, newRootSide), compareValue, logID)
	}
	btpUnlock(newRoot, logID)
	semaphoreLockCount--
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func BTPGetValue(binaryTree *BinaryTreeParallel) interface{} {
	if binaryTree != nil && binaryTree.value != nil {
		return binaryTree.value
	}
	return -1
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
