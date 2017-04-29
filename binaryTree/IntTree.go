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
	localIntArray    [][]int
	localExtraArray  [][]interface{}
}

func (iom *intOperationManager) StoreValue() int {
	if iom.valueIndex == -1 {
		iom.valueIndex = int(atomic.AddInt32(&iom.it.nextIndex, 1) - 1)
		if len(iom.localIntArray) <= iom.valueIndex>>8 {
			iom.it.rwmutex.Lock()
			freeListLength := len(iom.it.freeList)
			if freeListLength > 1 {
				iom.valueIndex = iom.it.freeList[freeListLength-1]
				iom.it.freeList = iom.it.freeList[:freeListLength-1]
			} else {
				if len(iom.it.ints) <= iom.valueIndex>>8 {
					iom.it.ints = append(iom.it.ints, make([]int, 256))
					if iom.it.StoreExtraData {
						iom.it.extraData = append(iom.it.extraData, make([]interface{}, 256))
					}
				}
				iom.localIntArray = iom.it.ints
				iom.localExtraArray = iom.it.extraData
			}
			iom.it.rwmutex.Unlock()
		}
		iom.localIntArray[iom.valueIndex>>8][iom.valueIndex&255] = iom.value
		iom.localIntArray[iom.valueIndex>>8][iom.valueIndex&255] = iom.value
	}
	return iom.valueIndex
}

func (iom *intOperationManager) getStoredValue(valueIndex int) int {
	if len(iom.localIntArray) <= valueIndex>>8 {
		iom.it.rwmutex.RLock()
		iom.localIntArray = iom.it.ints
		iom.localExtraArray = iom.it.extraData
		iom.it.rwmutex.RUnlock()
	}
	return iom.localIntArray[valueIndex>>8][valueIndex&255]
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
	if valueIndex == iom.valueIndex {
		return
	}
	storedValue := iom.getStoredValue(valueIndex)
	if iom.value == storedValue {
		if iom.valueIndex == -1 {
			iom.valueIndex = valueIndex
		} else if iom.valueIndex != valueIndex {
			panic("value stored twice")
		}
	} else if iom.value > storedValue {
		comparisonResult = 1
	} else {
		comparisonResult = -1
	}
	return
}

func (iom *intOperationManager) GetValueString() string {
	return fmt.Sprint(iom.value)
}

func (iom *intOperationManager) GetStoredValueString(valueIndex int) string {
	if valueIndex < 0 {
		return "nil"
	}
	storedValue := iom.getStoredValue(valueIndex)
	return fmt.Sprint(storedValue)
}

func (iom *intOperationManager) SetValueToStoredValue(valueIndex int) {
	iom.value = iom.getStoredValue(valueIndex)
	if iom.it.StoreExtraData {
		iom.extraData = iom.localExtraArray[valueIndex>>8][valueIndex&255]
	}
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
	ints           [][]int
	nextIndex      int32
	extraData      [][]interface{}
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
