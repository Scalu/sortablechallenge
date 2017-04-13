package sortablechallengeutils

import (
	"fmt"
	"sync"
)

type binaryTreeParrallelNode struct {
	value           int
	weight          int
	possibleInserts int
	parent          *BinaryTreeParrallel
	leftright       [2]*BinaryTreeParrallel
}

// BinaryTreeParrallel creates a binary tree that supports concurrent inserts and searches
// use BTPGetNew() to properly create one
type BinaryTreeParrallel struct {
	mutexes [2]sync.Mutex // 1: used by rebalance once possible inserts is 0, 2nd pass of insert, and searches
	// 0:used by rebalance when scheduled, and first pass of insert
	node *binaryTreeParrallelNode
}

func btpLock(binaryTree *BinaryTreeParrallel, mutexIndex int, logID string) {
	var parent, left, right *BinaryTreeParrallel
	var weight int
	if binaryTree.node != nil {
		parent = binaryTree.node.parent
		left = binaryTree.node.leftright[0]
		right = binaryTree.node.leftright[1]
		weight = binaryTree.node.weight
	}
	fmt.Printf("%s Locking   %d btp %d, parent %d, left %d, right %d, weight %d\n", logID, mutexIndex, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
	binaryTree.mutexes[mutexIndex].Lock()
	fmt.Printf("%s Locked    %d btp %d, parent %d, left %d, right %d, weight %d\n", logID, mutexIndex, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
}

func btpUnlock(binaryTree *BinaryTreeParrallel, mutexIndex int, logID string) {
	var parent, left, right *BinaryTreeParrallel
	var weight int
	if binaryTree.node != nil {
		parent = binaryTree.node.parent
		left = binaryTree.node.leftright[0]
		right = binaryTree.node.leftright[1]
		weight = binaryTree.node.weight
	}
	fmt.Printf("%s Unlocking %d btp %d, parent %d, left %d, right %d, weight %d\n", logID, mutexIndex, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
	binaryTree.mutexes[mutexIndex].Unlock()
	fmt.Printf("%s Unlocked  %d btp %d, parent %d, left %d, right %d, weight %d\n", logID, mutexIndex, BTPGetValue(binaryTree), BTPGetValue(parent), BTPGetValue(left), BTPGetValue(right), weight)
}

// BTPSearch call with a go routing for concurrency
func BTPSearch(binaryTree *BinaryTreeParrallel, compareValue func(int) int, previousLock *sync.Mutex, onRebalance func(), logID string) *BinaryTreeParrallel {
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprint(logID, "BTPSearch did not release all of it's locks"))
		}
	}()
	if binaryTree == nil {
		panic(fmt.Sprint(logID, "BTPSearch should not be called with a nil binaryTree value"))
	}
	btpLock(binaryTree, 1, logID) // lock the current tree
	semaphoreLockCount++
	if previousLock != nil {
		previousLock.Unlock()
	}
	for binaryTree.node != nil {
		compareResult := compareValue(binaryTree.node.value)
		nextBinaryTree := binaryTree.node.leftright[0]
		switch compareResult {
		case -1:
		case 1:
			nextBinaryTree = binaryTree.node.leftright[1]
		case 0:
			btpUnlock(binaryTree, 1, logID)
			semaphoreLockCount--
			return binaryTree
		}
		btpLock(nextBinaryTree, 1, logID)
		semaphoreLockCount++
		btpUnlock(binaryTree, 1, logID)
		semaphoreLockCount--
		binaryTree = nextBinaryTree
	}
	btpUnlock(binaryTree, 1, logID)
	semaphoreLockCount--
	return nil
}

// BTPInsert call with a go routine for concurrency
func BTPInsert(binaryTree *BinaryTreeParrallel, parent *BinaryTreeParrallel, getInsertValue func() int, compareValue func(int) int, previousLock *sync.Mutex, onRebalance func(), logID string) (returnValue int) {
	semaphoreLockCount := 0
	newNodeAdded := false
	returnValue = -1
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprint(logID, " BTPInsert did not release all of it's locks"))
		}
	}()
	if binaryTree == nil {
		panic(fmt.Sprint(logID, " BTPInsert should not be called with a nil binaryTree value"))
	}
	for pass := 0; pass < 2; pass++ {
		binaryTree := binaryTree
		btpLock(binaryTree, pass, logID)
		semaphoreLockCount++
		if previousLock != nil {
			previousLock.Unlock()
			previousLock = nil
		}
	iterationLoop:
		for binaryTree.node != nil {
			compareResult := compareValue(binaryTree.node.value)
			nextBinaryTree := binaryTree.node.leftright[0]
			switch compareResult {
			case -1:
			case 1:
				nextBinaryTree = binaryTree.node.leftright[1]
			case 0:
				break iterationLoop
			}
			if pass == 0 {
				binaryTree.node.possibleInserts++
			} else {
				if newNodeAdded {
					binaryTree.node.weight += compareResult
				}
				binaryTree.node.possibleInserts--
				if binaryTree.node.weight+binaryTree.node.possibleInserts < -1 || binaryTree.node.weight-binaryTree.node.possibleInserts > 1 {
					go btpRebalance(binaryTree, onRebalance, logID)
				}
			}
			btpLock(nextBinaryTree, pass, logID)
			semaphoreLockCount++
			btpUnlock(binaryTree, pass, logID)
			semaphoreLockCount--
			parent = binaryTree
			binaryTree = nextBinaryTree
		}
		if binaryTree.node == nil {
			if pass != 0 {
				panic(fmt.Sprintf("%s Error did not find inserted value on second pass", logID))
			}
			binaryTree.node = &binaryTreeParrallelNode{value: getInsertValue(), parent: parent}
			binaryTree.node.leftright[0] = &BinaryTreeParrallel{}
			binaryTree.node.leftright[1] = &BinaryTreeParrallel{}
			newNodeAdded = true
		}
		returnValue = binaryTree.node.value
		btpUnlock(binaryTree, pass, logID)
		semaphoreLockCount--
	}
	return
}

// btpRebalance call with a goroutine
func btpRebalance(binaryTree *BinaryTreeParrallel, onRebalance func(), logID string) {
	logID = fmt.Sprintf("%s rebalance", logID)
	semaphoreLockCount := 0
	defer func() {
		if semaphoreLockCount > 0 {
			panic(fmt.Sprintf("%s btpRebalance did not release all of it's locks", logID))
		}
	}()
	btpLock(binaryTree, 0, logID)
	semaphoreLockCount++
	defer func() {
		btpUnlock(binaryTree, 0, logID)
		semaphoreLockCount--
	}()
	for binaryTree.node.possibleInserts > 0 { // do nothing until inserts have resolved
	}
	if binaryTree.node.weight > -2 && binaryTree.node.weight < 2 {
		return
	}
	btpLock(binaryTree, 1, logID)
	semaphoreLockCount++
	defer func() {
		btpUnlock(binaryTree, 1, logID)
		semaphoreLockCount--
	}()
	if onRebalance != nil {
		onRebalance()
	}
	node := binaryTree.node
	newRootSide := 0           // side from which the new root node is being taken from
	newRootSideWeightMod := -1 // weight modifier for nodes between the current and new roots
	rootNewSide := 1           // side where the old root node is being moved to
	if node.weight > 0 {       // swap sides if the weight is positive
		newRootSide = 1
		newRootSideWeightMod = 1
		rootNewSide = 0
	}

	// get the new root
	newRoot := node.leftright[newRootSide]
	btpLock(newRoot, 1, logID)
	semaphoreLockCount++
	for newRoot.node.leftright[rootNewSide].node != nil {
		newRoot.node.weight += newRootSideWeightMod            // modify the weight to account for the new root node being removed
		btpLock(newRoot.node.leftright[rootNewSide], 1, logID) // lock the next newRoot candidate
		semaphoreLockCount++
		if newRoot.node.weight < -1 || newRoot.node.weight > 1 {
			go btpRebalance(newRoot, onRebalance, logID) // rebalance if required
		}
		btpUnlock(newRoot, 1, logID) // unlock the old newRoot
		semaphoreLockCount--
		newRoot = newRoot.node.leftright[rootNewSide] // assign the new newRoot
	}
	defer func() {
		btpUnlock(newRoot, 1, logID)
		semaphoreLockCount--
	}()

	// get the old root's new parent
	oldRootNewParent := node.leftright[rootNewSide]
	btpLock(oldRootNewParent, 1, logID) // lock the old root's new parent
	semaphoreLockCount++
	for oldRootNewParent.node != nil {
		oldRootNewParent.node.weight += newRootSideWeightMod // adjust the weight to account for the old root being added
		if oldRootNewParent.node.leftright[newRootSide].node != nil {
			btpLock(oldRootNewParent.node.leftright[newRootSide], 1, logID) // lock the next oldRootNewParent candidate
			semaphoreLockCount++
			if oldRootNewParent.node.weight < -1 || oldRootNewParent.node.weight > 1 {
				go btpRebalance(oldRootNewParent, onRebalance, logID)
			}
			btpUnlock(oldRootNewParent, 1, logID) // unlock the old oldRootNewParent
			semaphoreLockCount--
			oldRootNewParent = oldRootNewParent.node.leftright[newRootSide] // assign the new oldRootNewParent candidate
		} else {
			break
		}
	}
	defer func() {
		btpUnlock(oldRootNewParent, 1, logID)
		semaphoreLockCount--
	}()
	// put the new root in the old root's place
	binaryTree.node = newRoot.node
	// cut out the new root if required
	if newRoot.node.parent != binaryTree {
		newRoot.node.parent.node.leftright[rootNewSide] = newRoot.node.leftright[newRootSide]
		if newRoot.node.leftright[newRootSide].node != nil {
			target := newRoot.node.leftright[newRootSide]
			btpLock(target, 1, logID)
			semaphoreLockCount++
			defer func() {
				btpUnlock(target, 1, logID)
				semaphoreLockCount--
			}()
			target.node.parent = binaryTree.node.parent
		}
		binaryTree.node.leftright[newRootSide] = node.leftright[newRootSide]
	}
	binaryTree.node.parent = node.parent
	binaryTree.node.weight = node.weight - 2*newRootSideWeightMod
	// check where the old root node goes
	if oldRootNewParent.node != nil {
		// new parent found for it
		binaryTree.node.leftright[rootNewSide] = node.leftright[rootNewSide]
		node.parent = oldRootNewParent
		oldRootNewParent.node.leftright[newRootSide].node = node
	} else {
		// new root node becomes it's parent
		binaryTree.node.leftright[rootNewSide].node = node
		node.parent = binaryTree
	}
	// old root should have no children
	node.leftright[0] = &BinaryTreeParrallel{}
	node.leftright[1] = &BinaryTreeParrallel{}
	node.weight = 0
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func BTPGetValue(binaryTree *BinaryTreeParrallel) int {
	if binaryTree != nil && binaryTree.node != nil {
		return binaryTree.node.value
	}
	return -1
}

// BTPGetNext returns the next BinaryTreeParrallel object, returns nil when it reaches the end
// not safe while values are being concurrently inserted
func BTPGetNext(binaryTree *BinaryTreeParrallel) *BinaryTreeParrallel {
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
func BTPGetFirst(binaryTree *BinaryTreeParrallel) *BinaryTreeParrallel {
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
