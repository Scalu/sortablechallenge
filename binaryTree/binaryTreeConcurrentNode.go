package binaryTree

import (
	"fmt"
	"sync"
)

// btpMutex a Mutex wrapper that helps to keep track of which process has a specific Mutex locked
type btpMutex struct {
	mutex sync.RWMutex
	node  *binaryTreeConcurrentNode
}

type btpLockedMutex struct {
	isRLocked bool
	isWLocked bool
	mutex     *btpMutex
}

// lock logs a message if the Mutex is already locked before locking the Mutex, and then logs when the Mutex is locked
func (mutex *btpMutex) wlock(logFunction BtpLogFunction, btom OperationManager, lockedMutex *btpLockedMutex) {
	if logFunction("") {
		logFunction(fmt.Sprintf("write locking node %s", btpGetNodeString(mutex.node, btom)))
	}
	mutex.mutex.Lock()
	if logFunction("") {
		logFunction(fmt.Sprintf("write locked node %s", btpGetNodeString(mutex.node, btom)))
	}
	lockedMutex.mutex = mutex
	lockedMutex.isWLocked = true
}

// unlock logs a message after the Mutex is unlocked
func (mutex *btpLockedMutex) wunlock(logFunction BtpLogFunction, btom OperationManager) {
	if !mutex.isWLocked {
		panic("Unlocking node for writing that wasn't write-locked")
	}
	mutex.mutex.mutex.Unlock()
	mutex.isWLocked = false
	if logFunction("") {
		logFunction(fmt.Sprintf("write unlocked node %s", btpGetNodeString(mutex.mutex.node, btom)))
	}
}

// lock logs a message if the Mutex is already locked before locking the Mutex, and then logs when the Mutex is locked
func (mutex *btpMutex) rlock(logFunction BtpLogFunction, btom OperationManager, lockedMutex *btpLockedMutex) {
	if logFunction("") {
		logFunction(fmt.Sprintf("read locking node %s", btpGetNodeString(mutex.node, btom)))
	}
	mutex.mutex.RLock()
	if logFunction("") {
		logFunction(fmt.Sprintf("read locked node %s", btpGetNodeString(mutex.node, btom)))
	}
	if lockedMutex != nil {
		lockedMutex.mutex = mutex
		lockedMutex.isRLocked = true
	}
}

// unlock logs a message after the Mutex is unlocked
func (mutex *btpLockedMutex) runlock(logFunction BtpLogFunction, btom OperationManager) {
	if !mutex.isRLocked {
		panic("Unlocking node for reading that wasn't read-locked")
	}
	mutex.mutex.mutex.RUnlock()
	mutex.isRLocked = false
	if logFunction("") {
		logFunction(fmt.Sprintf("read unlocked node %s", btpGetNodeString(mutex.mutex.node, btom)))
	}
}

// binaryTreeConcurrentNode creates a weight-balanced concurrent binary tree that supports parallel insert, delete and search processes
type binaryTreeConcurrentNode struct {
	mutex            btpMutex
	valueIndex       int
	currentWeight    int
	possibleWtAdjust [2]int // possible weight adjustments pending inserts and deletions
	parent           *binaryTreeConcurrentNode
	sideFromParent   int
	level            int
	leftright        [2]*binaryTreeConcurrentNode
	branchBoundaries [2]int
}

// OperationManager interface required by binary tree operations to store and compare values
type OperationManager interface {
	StoreValue() int                               // stores the operation value, can be called multiple times and should return the same index
	RestoreValue() int                             // copies the value to a new location and returns the index
	DeleteValue()                                  // deletes value from the storage
	CompareValueTo(int) int                        // compares set value to the stored value
	GetValueString() string                        // gets a string for the operation value
	GetStoredValueString(int) string               // gets a string for the stored value
	SetValueToStoredValue(int)                     // sets the value to a stored value, allows reuse
	HandleResult(int, bool)                        // handles the result of the operation
	OnRebalance()                                  // handles tracking rebalance count if necessary
	StartRebalance() bool                          // store the value and valueIndex and return true on first call, or just return false
	EndRebalance()                                 // restore the value and valueIndex, will panic if called with StartRebalance being previously called
	GetClone() OperationManager                    // returns a clone, used for rebalancing
	AddToRebalanceList(*binaryTreeConcurrentNode)  // adds a node to a list of nodes to be rebalanced
	GetNodeToRebalance() *binaryTreeConcurrentNode // returns next node in rebalance list, or nil if list is empty
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
func btpGetWeight(node *binaryTreeConcurrentNode, currentLock *btpLockedMutex, logFunction BtpLogFunction, btom OperationManager) (currentWeight int) {
	var lockedMutex btpLockedMutex
	if currentLock == nil || !currentLock.isRLocked && !currentLock.isWLocked {
		node.mutex.rlock(logFunction, btom, &lockedMutex)
	}
	currentWeight = node.currentWeight
	if lockedMutex.isRLocked {
		lockedMutex.runlock(logFunction, btom)
	}
	return
}

// btpAdjustPossibleWtAdj Locks the weight mutex and then adjusts one of the possible weight adjustment values by the given amount
// it unlocks the weight mutex when it's done
func btpAdjustPossibleWtAdj(node *binaryTreeConcurrentNode, currentLock *btpLockedMutex, side int, amount int, logFunction BtpLogFunction, btom OperationManager) {
	var lockedMutex btpLockedMutex
	if currentLock == nil || !currentLock.isWLocked && !currentLock.isRLocked {
		currentLock = &lockedMutex
		node.mutex.wlock(logFunction, btom, &lockedMutex)
	} else if currentLock.isRLocked {
		panic("cannot adjust possible weights with a read-only lock")
	}
	node.possibleWtAdjust[side] += amount
	btpAddToRebalanceIfNeeded(node, currentLock, btom)
	if lockedMutex.isWLocked {
		lockedMutex.wunlock(logFunction, btom)
	}
}

func btpAddToRebalanceIfNeeded(node *binaryTreeConcurrentNode, currentLock *btpLockedMutex, btom OperationManager) {
	if currentLock == nil || !currentLock.isWLocked && !currentLock.isRLocked {
		panic("cannot check unlocked node for rebalancing")
	}
	if node.currentWeight+node.possibleWtAdjust[1] < -1 || node.currentWeight-node.possibleWtAdjust[0] > 1 {
		btom.AddToRebalanceList(node)
	}
}

// btpAdjustWeightAndPossibleWtAdj Locks the weight mutex, adjusts weight values, and then unlocks the weight mutex when it's done
// weight is adjusted by given amount and then corresponding possible weight adjustment value is decreased
func btpAdjustWeightAndPossibleWtAdj(node *binaryTreeConcurrentNode, currentLock *btpLockedMutex, amount int, logFunction BtpLogFunction, btom OperationManager) {
	var lockedMutex btpLockedMutex
	if currentLock == nil || !currentLock.isWLocked && !currentLock.isRLocked {
		node.mutex.wlock(logFunction, btom, &lockedMutex)
		currentLock = &lockedMutex
		defer lockedMutex.wunlock(logFunction, btom)
	} else if currentLock.isRLocked {
		panic("cannot adjust curreent and possible weights with a read-only lock")
	}
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
	btpAddToRebalanceIfNeeded(node, currentLock, btom)
}

func btpGetNodePath(node *binaryTreeConcurrentNode) string {
	if node.parent != nil {
		return fmt.Sprint(btpGetNodePath(node.parent), node.sideFromParent)
	}
	return fmt.Sprint(node.sideFromParent)
}

// btpGetNodeString returns a string representation of the node used for logging
func btpGetNodeString(node *binaryTreeConcurrentNode, btom OperationManager) string {
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
	parentNodeValueIndex := -1
	if node.parent != nil {
		parentNodeValueIndex = node.parent.valueIndex
	}
	leftNodeValueIndex := -1
	if node.leftright[0] != nil {
		leftNodeValueIndex = node.leftright[0].valueIndex
	}
	rightNodeValueIndex := -1
	if node.leftright[1] != nil {
		rightNodeValueIndex = node.leftright[1].valueIndex
	}
	return fmt.Sprintf("btp %s, value %s i%d, parent %s i%d, left %s i%d, right %s i%d, branch bounds %s i%d - %s i%d, weight %d, possible weight mods -%d +%d",
		btpGetNodePath(node), btpGetValue(node, btom), node.valueIndex, btpGetValue(node.parent, btom), parentNodeValueIndex,
		btpGetValue(node.leftright[0], btom), leftNodeValueIndex, btpGetValue(node.leftright[1], btom), rightNodeValueIndex,
		branchBoundaryStrings[0], node.branchBoundaries[0], branchBoundaryStrings[1], node.branchBoundaries[1],
		node.currentWeight, node.possibleWtAdjust[0], node.possibleWtAdjust[1])
}

// btpGetBranchBoundary locks the Mutex, returns the value, and unlocks the Mutex for the corresponding boundary side
func btpGetBranchBoundary(node *binaryTreeConcurrentNode, currentLock *btpLockedMutex, side int, logFunction BtpLogFunction, btom OperationManager) (valueIndex int) {
	var lockedMutex btpLockedMutex
	if currentLock == nil || !currentLock.isWLocked && !currentLock.isRLocked {
		node.mutex.rlock(logFunction, btom, &lockedMutex)
		defer lockedMutex.runlock(logFunction, btom)
	}
	return node.branchBoundaries[side]
}

func btpNextStep(node *binaryTreeConcurrentNode, currentLock *btpLockedMutex, comparisonResult int, logFunction BtpLogFunction, btom OperationManager) (nextNode *binaryTreeConcurrentNode, matchFound bool) {
	var newLock btpLockedMutex
	switch comparisonResult {
	case -1:
		nextNode = node.leftright[0]
	case 1:
		nextNode = node.leftright[1]
	case 0:
		nextNode = node
		matchFound = true
		return
	default:
		panic(fmt.Sprintf("compareValues function returned invalid comparison value %d", comparisonResult))
	}
	if currentLock != nil {
		if currentLock.isRLocked {
			nextNode.mutex.rlock(logFunction, btom, &newLock)
			currentLock.runlock(logFunction, btom)
		} else if currentLock.isWLocked {
			nextNode.mutex.wlock(logFunction, btom, &newLock)
			currentLock.wunlock(logFunction, btom)
		} else {
			panic("btpNextStep requires locked node")
		}
		*currentLock = newLock
	}
	return
}

// btpSearch Returns the value if it is found, or nil if the value was not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpSearch(node *binaryTreeConcurrentNode, onFirstLock func(), logFunction BtpLogFunction, btom OperationManager) {
	if node == nil {
		logFunction("BTPSearch should not be called with a nil binaryTree value")
		panic("BTPSearch should not be called with a nil binaryTree value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPSearch ", btom.GetValueString()))
	}
	var nodeLockStore btpLockedMutex
	nodeLock := &nodeLockStore
	node.mutex.rlock(logFunction, btom, nodeLock) // lock the current tree
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	for node.valueIndex > -1 && !matchFound {
		node, matchFound = btpNextStep(node, nodeLock, btom.CompareValueTo(node.valueIndex), logFunction, btom)
	}
	btom.HandleResult(node.valueIndex, matchFound)
	nodeLock.runlock(logFunction, btom)
}

type btpAdjustNodeElement struct {
	node *binaryTreeConcurrentNode
	side int
	lock btpLockedMutex
}

// btpInsert Returns the value to be inserted, and wether a match was found with an existing value and thus no insertion was required.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpInsert(node *binaryTreeConcurrentNode, btom OperationManager, onFirstLock func(), logFunction BtpLogFunction) {
	if node == nil {
		logFunction("BTPInsert should not be called with a nil binaryTree value")
		panic("BTPInsert should not be called with a nil binaryTree value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPInsert ", btom.GetValueString()))
	}
	adjustWeights := make([]btpAdjustNodeElement, 0, 4)
	var nodeLockStore btpLockedMutex
	nodeLock := &nodeLockStore
	node.mutex.wlock(logFunction, btom, nodeLock) // lock the current tree
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	var valueKnownToNotExist bool
	var resultIndex int
	for node.valueIndex > -1 && !matchFound {
		comparisonResult := btom.CompareValueTo(node.valueIndex)
		if comparisonResult != 0 {
			sideIndex := 0
			movingTowardsImbalance := node.currentWeight+node.possibleWtAdjust[1] < 0
			if comparisonResult > 0 {
				sideIndex = 1
				movingTowardsImbalance = node.currentWeight-node.possibleWtAdjust[0] > 0
			}
			// compare value to boundary
			comparisonToBoundryResult := btom.CompareValueTo(node.branchBoundaries[sideIndex])
			if comparisonToBoundryResult == comparisonResult {
				node.branchBoundaries[sideIndex] = btom.StoreValue()
				valueKnownToNotExist = true
			} else if comparisonToBoundryResult == 0 {
				matchFound = true
				resultIndex = node.branchBoundaries[sideIndex]
				break
			}
			var nextValue int
			// check if self-balancing possible
			if movingTowardsImbalance {
				nextValue = btpGetBranchBoundary(node.leftright[sideIndex], nil, 1-sideIndex, logFunction, btom)
				if nextValue == -1 {
					movingTowardsImbalance = false
				}
			}
			if movingTowardsImbalance {
				if btom.CompareValueTo(nextValue) == 0-comparisonResult {
					// replace current node, and continue insert with replaced value in the opposite direction
					currentNodeValueIndex := node.valueIndex
					node.valueIndex = btom.StoreValue()
					valueKnownToNotExist = true
					btom.SetValueToStoredValue(currentNodeValueIndex)
					comparisonResult = 0 - comparisonResult
					sideIndex = 1 - sideIndex
				}
			}
			// set possible weight adjustment
			if valueKnownToNotExist {
				node.currentWeight += sideIndex*2 - 1
				btpAddToRebalanceIfNeeded(node, nodeLock, btom)
			} else {
				btpAdjustPossibleWtAdj(node, nodeLock, sideIndex, 1, logFunction, btom)
				adjustWeights = append(adjustWeights, btpAdjustNodeElement{node: node, side: sideIndex})
			}
			node, matchFound = btpNextStep(node, nodeLock, comparisonResult, logFunction, btom)
		} else {
			matchFound = true
			resultIndex = node.valueIndex
		}
	}
	if node.valueIndex == -1 {
		node.valueIndex = btom.StoreValue()
		node.branchBoundaries = [2]int{node.valueIndex, node.valueIndex}
		node.leftright[0] = &binaryTreeConcurrentNode{parent: node, valueIndex: -1, branchBoundaries: [2]int{-1, -1}, sideFromParent: 0, level: node.level + 1}
		node.leftright[1] = &binaryTreeConcurrentNode{parent: node, valueIndex: -1, branchBoundaries: [2]int{-1, -1}, sideFromParent: 1, level: node.level + 1}
		node.mutex.node = node
		resultIndex = node.valueIndex
	}
	nodeLock.wunlock(logFunction, btom)
	btom.HandleResult(resultIndex, matchFound)
	for _, adjustWeight := range adjustWeights {
		if !matchFound {
			btpAdjustWeightAndPossibleWtAdj(adjustWeight.node, nil, adjustWeight.side*2-1, logFunction, btom)
		} else {
			btpAdjustPossibleWtAdj(adjustWeight.node, nil, adjustWeight.side, -1, logFunction, btom)
		}
		if logFunction("") {
			logFunction(fmt.Sprintf("adjusting weights %s", btpGetNodeString(adjustWeight.node, btom)))
		}
	}
}

// btpDelete Deletes the given value from the given tree.
// Throws a panic if mustMatch is set to true and a matching value is not found.
// The previousLock when specified, gets unlocked when a lock on the given binary tree node is acquired.
// this can be used to set the order of searches inserts and deletes goroutines on the tree.
func btpDelete(node *binaryTreeConcurrentNode, btom OperationManager, onFirstLock func(), mustMatch bool, keepValueStored bool, logFunction BtpLogFunction) {
	if node == nil {
		logFunction("BTPDelete should not be called with a nil node value")
		panic("BTPDelete should not be called with a nil node value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPDelete", btom.GetValueString()))
	}
	adjustWeights := make([]btpAdjustNodeElement, 0, 4)
	adjustChildBounds := make([]btpAdjustNodeElement, 0, 4)
	var nodeLockStore btpLockedMutex
	nodeLock := &nodeLockStore
	node.mutex.wlock(logFunction, btom, nodeLock) // lock the current tree
	if onFirstLock != nil {
		onFirstLock()
	}
	var matchFound bool
	var closestValues [2]int
	var keepLock bool
	for node.valueIndex > -1 && !matchFound {
		comparisonResult := btom.CompareValueTo(node.valueIndex)
		if comparisonResult != 0 {
			sideToDeleteFrom := 0
			if comparisonResult > 0 {
				sideToDeleteFrom = 1
			}
			// adjust weights
			btpAdjustPossibleWtAdj(node, nodeLock, 1-sideToDeleteFrom, 1, logFunction, btom)
			adjustWeights = append(adjustWeights, btpAdjustNodeElement{node: node, side: 1 - sideToDeleteFrom})
			// check if branchBounds need to be adjusted
			keepLock = btom.CompareValueTo(node.branchBoundaries[sideToDeleteFrom]) == 0
			if keepLock {
				mustMatch = true
				adjustChildBounds = append(adjustChildBounds, btpAdjustNodeElement{node: node, side: sideToDeleteFrom, lock: *nodeLock})
			}
			// adjust closestValues
			closestValues[1-sideToDeleteFrom] = node.valueIndex
		}
		node, matchFound = btpNextStep(node, nil, comparisonResult, logFunction, btom)
		if !matchFound {
			var newNodeLock btpLockedMutex
			node.mutex.wlock(logFunction, btom, &newNodeLock)
			if !keepLock {
				nodeLock.wunlock(logFunction, btom)
			}
			*nodeLock = newNodeLock
		}
	}
	var nodeToDeleteFrom *binaryTreeConcurrentNode
	var replacementValueIndex int
	if matchFound {
		// adjust closest values
		var leftLock, rightLock btpLockedMutex
		node.leftright[0].mutex.rlock(logFunction, btom, &leftLock)
		node.leftright[1].mutex.rlock(logFunction, btom, &rightLock)
		if node.leftright[0].valueIndex > -1 {
			closestValues[0] = btpGetBranchBoundary(node.leftright[0], &leftLock, 1, logFunction, btom)
		}
		if node.leftright[1].valueIndex > -1 {
			closestValues[1] = btpGetBranchBoundary(node.leftright[1], &rightLock, 0, logFunction, btom)
		}
		// adjust bounds
		for _, nodeAndSide := range adjustChildBounds {
			nodeAndSide.node.branchBoundaries[nodeAndSide.side] = closestValues[1-nodeAndSide.side]
			nodeAndSide.lock.wunlock(logFunction, btom)
			if logFunction("") {
				logFunction(fmt.Sprintf("adjusted boundaries %s", btpGetNodeString(nodeAndSide.node, btom)))
			}
		}
		// remove it
		if !keepValueStored {
			btom.DeleteValue()
		}
		if node.leftright[0].valueIndex == -1 && node.leftright[1].valueIndex == -1 {
			node.valueIndex = -1
			node.leftright[0] = nil
			node.leftright[1] = nil
			node.branchBoundaries[0] = -1
			node.branchBoundaries[1] = -1
			if logFunction("") {
				logFunction(fmt.Sprintf("deleted leaf %s", btpGetNodeString(node, btom)))
			}
		} else {
			// get the side to delete from, waiting for weight adjustments if necessary
			sideToDeleteFrom := -1
			if node.leftright[0].valueIndex == -1 ||
				node.leftright[1].valueIndex != -1 &&
					node.currentWeight-node.possibleWtAdjust[0] >= 0-node.possibleWtAdjust[1]-node.currentWeight {
				node.currentWeight--
				sideToDeleteFrom = 1
			} else {
				node.currentWeight++
				sideToDeleteFrom = 0
			}
			btpAddToRebalanceIfNeeded(node, nodeLock, btom)
			nodeToDeleteFrom = node.leftright[sideToDeleteFrom]
			// update with new value
			replacementValueIndex = closestValues[sideToDeleteFrom]
			if replacementValueIndex == -1 {
				logFunction("Delete should not replace a deleted value that has one or more branches with a nil value")
				panic("Delete should not replace a deleted value that has one or more branches with a nil value")
			}
			if node.branchBoundaries[1-sideToDeleteFrom] == node.valueIndex {
				node.branchBoundaries[1-sideToDeleteFrom] = replacementValueIndex
			}
			node.valueIndex = replacementValueIndex
			if logFunction("") {
				logFunction(fmt.Sprintf("deleted branching node %s", btpGetNodeString(node, btom)))
			}
		}
		leftLock.runlock(logFunction, btom)
		rightLock.runlock(logFunction, btom)
	} else {
		for _, nodeAndSide := range adjustChildBounds {
			nodeAndSide.lock.wunlock(logFunction, btom)
		}
		logFunction("node to delete not found")
	}
	if nodeToDeleteFrom != nil {
		// delete new value from old location in new process
		btom.SetValueToStoredValue(replacementValueIndex)
		btpDelete(nodeToDeleteFrom, btom, func() {
			nodeLock.wunlock(logFunction, btom)
		}, true, true, logFunction)
	} else {
		nodeLock.wunlock(logFunction, btom)
	}
	btom.HandleResult(-1, matchFound)
	for _, adjustWeight := range adjustWeights {
		if matchFound {
			btpAdjustWeightAndPossibleWtAdj(adjustWeight.node, nil, adjustWeight.side*2-1, logFunction, btom)
		} else {
			btpAdjustPossibleWtAdj(adjustWeight.node, nil, adjustWeight.side, -1, logFunction, btom)
		}
		if logFunction("") {
			logFunction(fmt.Sprintf("adjusting weights %s", btpGetNodeString(adjustWeight.node, btom)))
		}
	}
	if !matchFound && mustMatch {
		logFunction("Failed to delete when value was known to exist")
		panic("Failed to delete when value was known to exist")
	}
}

// btpRebalance needs to be changed(?) to a delete of the value being rebalanced followed by an insert of that value
func btpRebalance(node *binaryTreeConcurrentNode, logFunction BtpLogFunction, btom OperationManager) {
	if node == nil {
		logFunction("btpRebalance called on a nil value")
		panic("btpRebalance called on a nil value")
	}
	if logFunction("") {
		logFunction = btpSetLogFunction(logFunction, fmt.Sprint("BTPRebalance", btom.GetStoredValueString(node.valueIndex)))
	}
	var nodeLockStore btpLockedMutex
	nodeLock := &nodeLockStore
	node.mutex.wlock(logFunction, btom, nodeLock) // lock the current tree
	for node.currentWeight+node.possibleWtAdjust[1] < -1 || node.currentWeight-node.possibleWtAdjust[0] > 1 {
		btom.OnRebalance()
		newRootSide := 0            // side from which the new root node is being taken from
		if node.currentWeight > 0 { // swap sides if the weight is positive
			newRootSide = 1
		}
		rootNewSide := 1 - newRootSide // side where the old root node is being moved to

		// get the new root value
		newRootValueIndex := btpGetBranchBoundary(node.leftright[newRootSide], nil, rootNewSide, logFunction, btom)
		if newRootValueIndex == -1 {
			nodeLock.wunlock(logFunction, btom)
			logFunction("BTPRebalance should not replace root's value with a nil value")
			panic("BTPRebalance should not replace root's value with a nil value")
		}

		// adjust the node
		valueIndexToInsert := node.valueIndex
		node.valueIndex = newRootValueIndex
		node.currentWeight += 4*rootNewSide - 2

		// insert the oldRootValue on the rootNewSide
		btom.SetValueToStoredValue(valueIndexToInsert)
		btpInsert(node.leftright[rootNewSide], btom, nil, logFunction)

		// delete the newRootValue from the newRootSide
		btom.SetValueToStoredValue(newRootValueIndex)
		if node.currentWeight+node.possibleWtAdjust[1] < -1 || node.currentWeight-node.possibleWtAdjust[0] > 1 {
			btpDelete(node.leftright[newRootSide], btom, nil, true, true, logFunction)
		} else {
			btpDelete(node.leftright[newRootSide], btom, func() { nodeLock.wunlock(logFunction, btom) }, true, true, logFunction)
			break
		}
	}
	if nodeLock.isWLocked {
		nodeLock.wunlock(logFunction, btom)
	}
}

// BTPGetValue returns the value stored by a binary tree node
// not safe while values are being concurrently inserted
func btpGetValue(node *binaryTreeConcurrentNode, btom OperationManager) string {
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
