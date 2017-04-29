package binaryTree

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type intOperationManager struct {
	value            int
	valueIndex       int
	extraData        interface{}
	it               *IntTree
	handleResult     func(int, bool)
	rebalanceTorchID int32
}

func (iom *intOperationManager) StoreValue() int {
	if iom.valueIndex == -1 {
		iom.it.rwmutex.Lock()
		freeListLength := len(iom.it.freeList)
		if freeListLength > 1 {
			iom.valueIndex = iom.it.freeList[freeListLength-1]
			iom.it.freeList = iom.it.freeList[:freeListLength-1]
			iom.it.ints[iom.valueIndex] = iom.value
			if iom.it.StoreExtraData {
				iom.it.extraData[iom.valueIndex] = iom.extraData
			}
		} else {
			iom.valueIndex = len(iom.it.ints)
			iom.it.ints = append(iom.it.ints, iom.value)
			if iom.it.StoreExtraData {
				if len(iom.it.extraData) != iom.valueIndex {
					panic("Size of extraData array should match size of String array.")
				}
				iom.it.extraData = append(iom.it.extraData, iom.extraData)
			}
		}
		iom.it.rwmutex.Unlock()
	}
	return iom.valueIndex
}

func (iom *intOperationManager) DeleteValue() {
	if iom.valueIndex > -1 {
		iom.it.rwmutex.Lock()
		iom.it.freeList = append(iom.it.freeList, iom.valueIndex)
		iom.it.rwmutex.Unlock()
		iom.valueIndex = -1
	}
}

func (iom *intOperationManager) CompareValueTo(valueIndex int) (comparisonResult int) {
	iom.it.rwmutex.RLock()
	storedValue := iom.it.ints[valueIndex]
	iom.it.rwmutex.RUnlock()
	if iom.value < storedValue {
		return -1
	} else if iom.value > storedValue {
		return 1
	}
	if iom.valueIndex < 0 {
		iom.valueIndex = valueIndex
	} else if iom.valueIndex != valueIndex {
		panic("value stored twice")
	}
	return 0
}

func (iom *intOperationManager) GetValueString() string {
	return fmt.Sprint(iom.value)
}

func (iom *intOperationManager) GetStoredValueString(valueIndex int) string {
	if valueIndex < 0 {
		return "nil"
	}
	iom.it.rwmutex.RLock()
	storedValue := iom.it.ints[valueIndex]
	iom.it.rwmutex.RUnlock()
	return fmt.Sprint(storedValue)
}

func (iom *intOperationManager) SetValueToStoredValue(valueIndex int) {
	iom.it.rwmutex.RLock()
	iom.value = iom.it.ints[valueIndex]
	if iom.it.StoreExtraData {
		iom.extraData = iom.it.extraData[valueIndex]
	}
	iom.it.rwmutex.RUnlock()
	iom.valueIndex = valueIndex
}

func (iom *intOperationManager) HandleResult(matchIndex int, matchFound bool) {
	if iom.handleResult != nil {
		iom.handleResult(matchIndex, matchFound)
		iom.handleResult = nil
	}
}

func (iom *intOperationManager) OnRebalance() {
	iom.it.OnRebalance()
}

// AddToRebalanceList adds a node to a list of nodes to be rebalanced
func (iom *intOperationManager) AddToRebalanceList(node *binaryTreeConcurrentNode) {
	iom.it.btd.addToRebalanceList(node)
}

// GetNodeToRebalance returns next node in rebalance list, or nil if list is empty
func (iom *intOperationManager) GetNodeToRebalance() (node *binaryTreeConcurrentNode) {
	if iom.rebalanceTorchID == 0 {
		iom.rebalanceTorchID = atomic.AddInt32(&iom.it.rebalanceTorch, 1)
	} else if iom.rebalanceTorchID != atomic.LoadInt32(&iom.it.rebalanceTorch) {
		return
	}
	return iom.it.btd.getNodeToRebalance()
}

func (iom *intOperationManager) GetNewNode() *binaryTreeConcurrentNode {
	return iom.it.btd.getNewNode()
}

// IntTree a binary tree of string values
type IntTree struct {
	rwmutex        sync.RWMutex
	ints           []int
	extraData      []interface{}
	freeList       []int
	rootNode       *binaryTreeConcurrentNode
	StoreExtraData bool           // to be set if desired
	LogFunction    BtpLogFunction // to be set if desired
	OnRebalance    func()         // to be set if desired
	rebalanceTorch int32
	btd            binaryTreeData
}

func (it *IntTree) getRootNode() *binaryTreeConcurrentNode {
	if it.rootNode == nil {
		if it.LogFunction == nil {
			it.LogFunction = func(stringToLog string) bool {
				return false
			}
		}
		if it.OnRebalance == nil {
			it.OnRebalance = func() {}
		}
		it.rootNode = it.btd.getNewNode()
	}
	return it.rootNode
}

func (it *IntTree) SearchForValue(valueToFind int, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &intOperationManager{value: valueToFind, valueIndex: -1, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if it.StoreExtraData {
				extraData = it.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}, it: it}
	logFunction := btpSetLogFunction(it.LogFunction, logID)
	btpSearch(it.getRootNode(), onStart, logFunction, btom)
	it.btd.rebalanceNodes(logFunction, btom)
}

func (it *IntTree) InsertValue(valueToInsert int, extraData interface{}, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &intOperationManager{value: valueToInsert, valueIndex: -1, it: it, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if it.StoreExtraData {
				extraData = it.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}
	logFunction := btpSetLogFunction(it.LogFunction, logID)
	btpInsert(it.getRootNode(), btom, onStart, logFunction)
	it.btd.rebalanceNodes(logFunction, btom)
}

func (it *IntTree) DeleteValue(valueToDelete int, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &intOperationManager{value: valueToDelete, valueIndex: -1, it: it, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if it.StoreExtraData {
				extraData = it.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}
	logFunction := btpSetLogFunction(it.LogFunction, logID)
	btpDelete(it.getRootNode(), btom, onStart, false, false, logFunction)
	it.btd.rebalanceNodes(logFunction, btom)
}
