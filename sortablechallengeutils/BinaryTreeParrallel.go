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

// BinaryTreeParallel creates a binary tree that supports concurrent inserts, deletes and searches
type BinaryTreeParallel struct {
	mutex            sync.Mutex
	value            BinaryTreeValue
	weightMutex      sync.Mutex
	currentWeight    int
	possibleWtAdjust [2]int // possible weight adjustments pending inserts and deletions
	parent           *BinaryTreeParallel
	leftright        [2]*BinaryTreeParallel
	branchBoundaries [2]BinaryTreeValue
	branchBMutices   [2]sync.Mutex
	rebalancing      bool
}

func btpGetWeight(node *BinaryTreeParallel) int {
	node.weightMutex.Lock()
	defer node.weightMutex.Unlock()
	return node.currentWeight
}

func btpAdjustPossibleWtAjd(node *BinaryTreeParallel, side int, amount int) {
	node.weightMutex.Lock()
	defer node.weightMutex.Unlock()
	node.possibleWtAdjust[side] += amount
}

func btpAdjustWeightAndPossibleWtAdj(node *BinaryTreeParallel, amount int) {
	node.weightMutex.Lock()
	defer node.weightMutex.Unlock()
	node.currentWeight += amount
	if amount > 0 {
		node.possibleWtAdjust[1] -= amount
	} else {
		node.possibleWtAdjust[0] += amount
	}
}

func btpRebalanceIfNecessary(binaryTree *BinaryTreeParallel, onRebalance func(), logID *string) {
	binaryTree.weightMutex.Lock()
	defer binaryTree.weightMutex.Unlock()
	if !binaryTree.rebalancing &&
		(binaryTree.currentWeight+binaryTree.possibleWtAdjust[1] < -1 ||
			binaryTree.currentWeight-binaryTree.possibleWtAdjust[0] > 1) {
		binaryTree.rebalancing = true
		go btpRebalance(binaryTree, onRebalance, logID)
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

func btpGetBranchBoundary(node *BinaryTreeParallel, side int) BinaryTreeValue {
	node.branchBMutices[side].Lock()
	defer node.branchBMutices[side].Unlock()
	return node.branchBoundaries[side]
}

func btpAdjustChildBounds(node *BinaryTreeParallel, value BinaryTreeValue, logID *string) {
	if value == nil {
		return
	}
	comparisonResult := value.CompareTo(node.value)
	if comparisonResult == -1 {
		if value.CompareTo(btpGetBranchBoundary(node, 0)) == -1 {
			node.branchBoundaries[0] = value
		}
	} else {
		if value.CompareTo(btpGetBranchBoundary(node, 0)) == 1 {
			node.branchBoundaries[1] = value
		}
	}
}

func btpStep(binaryTree *BinaryTreeParallel, compareValues func(*BinaryTreeParallel, BinaryTreeValue) int, value BinaryTreeValue, logID *string) (nextStep *BinaryTreeParallel, matchFound bool) {
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
func BTPSearch(binaryTree *BinaryTreeParallel, value BinaryTreeValue, previousLock *sync.Mutex, logID *string) BinaryTreeValue {
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
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value BinaryTreeValue) int {
		return value.CompareTo(binaryTree.value)
	}, value, logID)
	if matchFound {
		return binaryTree.value
	}
	return nil
}

// BTPInsert call with a go routine for concurrency
func BTPInsert(binaryTree *BinaryTreeParallel, value BinaryTreeValue, previousLock *sync.Mutex, onRebalance func(), logID *string) BinaryTreeValue {
	if logID != nil {
		newLogString := fmt.Sprintf("%s Insert %s", *logID, value.ToString())
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
	binaryTree, matchFound = btpStep(binaryTree, func(binaryTree *BinaryTreeParallel, value BinaryTreeValue) (comparisonResult int) {
		comparisonResult = value.CompareTo(binaryTree.value)
		if comparisonResult != 0 {
			sideIndex := 0
			if comparisonResult > 0 {
				sideIndex = 1
			}
			btpAdjustPossibleWtAjd(binaryTree, sideIndex, 1)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				if !matchFound {
					btpAdjustWeightAndPossibleWtAdj(binaryTree, comparisonResult)
				} else {
					btpAdjustPossibleWtAjd(binaryTree, sideIndex, -1)
				}
				if logID != nil {
					fmt.Printf("%s adjusting weights %s\n", *logID, btpGetNodeString(binaryTree))
				}
				btpRebalanceIfNecessary(binaryTree, onRebalance, logID)
				prevAdjustWeights()
			}
			btpAdjustChildBounds(binaryTree, value, logID)
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
func BTPDelete(node *BinaryTreeParallel, value BinaryTreeValue, previousLock *sync.Mutex, onRebalance func(), mustMatch bool, logID *string) {
	if logID != nil {
		newLogString := fmt.Sprintf("%s Delete %s", *logID, value.ToString())
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
	var closestValues [2]BinaryTreeValue
	node, matchFound = btpStep(node, func(binaryTree *BinaryTreeParallel, value BinaryTreeValue) (comparisonResult int) {
		comparisonResult = value.CompareTo(node.value)
		if comparisonResult != 0 {
			sideToDeleteFrom := 0
			if comparisonResult > 0 {
				sideToDeleteFrom = 1
			}
			// adjust weights
			btpAdjustPossibleWtAjd(binaryTree, 1-sideToDeleteFrom, 1)
			prevAdjustWeights := adjustWeights
			adjustWeights = func() {
				if matchFound {
					btpAdjustWeightAndPossibleWtAdj(binaryTree, 0-comparisonResult)
				} else {
					btpAdjustPossibleWtAjd(binaryTree, 1-sideToDeleteFrom, -1)
				}
				if logID != nil {
					fmt.Printf("%s adjusting weights %s\n", *logID, btpGetNodeString(binaryTree))
				}
				btpRebalanceIfNecessary(binaryTree, onRebalance, logID)
				prevAdjustWeights()
			}
			// check if branchBounds need to be adjusted
			node.branchBMutices[sideToDeleteFrom].Lock()
			if value.CompareTo(node.branchBoundaries[sideToDeleteFrom]) == 0 {
				prevAdjustChildBounds := adjustChildBounds
				adjustChildBounds = func() {
					node.branchBoundaries[sideToDeleteFrom] = closestValues[1-sideToDeleteFrom]
					node.branchBMutices[sideToDeleteFrom].Unlock()
					if logID != nil {
						fmt.Printf("%s adjusting boundaries %s\n", *logID, btpGetNodeString(binaryTree))
					}
					prevAdjustChildBounds()
				}
			} else {
				node.branchBMutices[sideToDeleteFrom].Unlock()
			}
			// adjust closestValues
			closestValues[1-sideToDeleteFrom] = node.value
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
			node.leftright[0] = nil
			node.leftright[1] = nil
			node.branchBoundaries[0] = nil
			node.branchBoundaries[1] = nil
		} else {
			sideToDeleteFrom := 0
			node.weightMutex.Lock()
			if node.currentWeight > 0 {
				node.currentWeight--
				sideToDeleteFrom = 1
			} else {
				node.currentWeight++
			}
			node.weightMutex.Unlock()

			// update with new value
			node.value = btpGetBranchBoundary(node.leftright[sideToDeleteFrom], 1-sideToDeleteFrom)
			// update branch boundary if old value is one of them
			if node.leftright[1-sideToDeleteFrom].value == nil {
				node.branchBMutices[1-sideToDeleteFrom].Lock()
				node.branchBoundaries[1-sideToDeleteFrom] = node.value
				node.branchBMutices[1-sideToDeleteFrom].Unlock()
			}
			var deleteWaitMutex sync.Mutex
			// delete new value from old location, and wait until deletion starts before exiting
			deleteWaitMutex.Lock()
			go BTPDelete(node.leftright[sideToDeleteFrom], node.value, &deleteWaitMutex, onRebalance, true, logID)
			deleteWaitMutex.Lock()
		}
	} else if mustMatch {
		panic(fmt.Sprintf("%s Failed to delete a required deletion", *logID))
	}
}

// btpRebalance call with a goroutine
func btpRebalance(node *BinaryTreeParallel, onRebalance func(), logID *string) {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s btpRebalance did not release all of it's locks", *logID))
		}
	}()
	if node == nil || node.value == nil {
		panic(fmt.Sprintf("%s btpRebalance called on a nil value", *logID))
	}
	if logID != nil {
		rebalanceLogID := fmt.Sprintf("%s Rebalance", *logID)
		logID = &rebalanceLogID
	}
	btpLock(node, logID)
	semaphoreLockCount++
	if logID != nil {
		rebalanceLogID := fmt.Sprintf("%s %s", *logID, node.value.ToString())
		logID = &rebalanceLogID
	}
	defer func() {
		node.rebalancing = false
		btpRebalanceIfNecessary(node, onRebalance, logID)
		btpUnlock(node, logID)
		semaphoreLockCount--
	}()
	node.weightMutex.Lock()
	defer node.weightMutex.Unlock()
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
	newRootValue := btpGetBranchBoundary(newRoot, rootNewSide)

	var deleteStartedMutex, insertStartedMutex sync.Mutex
	// delete the newRootValue from the newRootSide
	deleteStartedMutex.Lock()
	go BTPDelete(node.leftright[newRootSide], newRootValue, &deleteStartedMutex, onRebalance, true, logID)

	// insert the oldRootValue on the rootNewSide
	insertStartedMutex.Lock()
	go BTPInsert(node.leftright[rootNewSide], node.value, &insertStartedMutex, onRebalance, logID)
	btpAdjustChildBounds(node, node.value, logID)

	// wait for the insert and delete to have started before continuing
	deleteStartedMutex.Lock()
	insertStartedMutex.Lock()

	// adjust the binaryTree
	node.value = newRootValue
	node.currentWeight += 2 * rootWeightMod
	node.rebalancing = false
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
