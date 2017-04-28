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
	backupValue      int
	backupIndex      int
	backupExtraData  interface{}
	rebalanceStarted bool
	st               *IntTree
	handleResult     func(int, bool)
	rebalanceTorchID int32
}

func (iom *intOperationManager) StoreValue() int {
	if iom.valueIndex == -1 {
		iom.st.rwmutex.Lock()
		freeListLength := len(iom.st.freeList)
		if freeListLength > 1 {
			iom.valueIndex = iom.st.freeList[freeListLength-1]
			iom.st.freeList = iom.st.freeList[:freeListLength-1]
			iom.st.ints[iom.valueIndex] = iom.value
			if iom.st.StoreExtraData {
				iom.st.extraData[iom.valueIndex] = iom.extraData
			}
		} else {
			iom.valueIndex = len(iom.st.ints)
			iom.st.ints = append(iom.st.ints, iom.value)
			if iom.st.StoreExtraData {
				if len(iom.st.extraData) != iom.valueIndex {
					panic("Size of extraData array should match size of String array.")
				}
				iom.st.extraData = append(iom.st.extraData, iom.extraData)
			}
		}
		iom.st.rwmutex.Unlock()
	}
	return iom.valueIndex
}

func (iom *intOperationManager) RestoreValue() int {
	iom.valueIndex = -1
	return iom.StoreValue()
}

func (iom *intOperationManager) DeleteValue() {
	if iom.valueIndex > -1 {
		iom.st.rwmutex.Lock()
		iom.st.freeList = append(iom.st.freeList, iom.valueIndex)
		iom.st.rwmutex.Unlock()
		iom.valueIndex = -1
	}
}

func (iom *intOperationManager) CompareValueTo(valueIndex int) (comparisonResult int) {
	iom.st.rwmutex.RLock()
	storedValue := iom.st.ints[valueIndex]
	iom.st.rwmutex.RUnlock()
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
	iom.st.rwmutex.RLock()
	storedValue := iom.st.ints[valueIndex]
	iom.st.rwmutex.RUnlock()
	return fmt.Sprint(storedValue)
}

func (iom *intOperationManager) SetValueToStoredValue(valueIndex int) {
	iom.st.rwmutex.RLock()
	iom.value = iom.st.ints[valueIndex]
	if iom.st.StoreExtraData {
		iom.extraData = iom.st.extraData[valueIndex]
	}
	iom.st.rwmutex.RUnlock()
	iom.valueIndex = valueIndex
}

func (iom *intOperationManager) HandleResult(matchIndex int, matchFound bool) {
	if iom.handleResult != nil {
		iom.handleResult(matchIndex, matchFound)
		iom.handleResult = nil
	}
}

func (iom *intOperationManager) OnRebalance() {
	iom.st.OnRebalance()
}

func (iom *intOperationManager) StartRebalance() bool {
	if iom.rebalanceStarted {
		return false
	}
	iom.backupIndex = iom.valueIndex
	iom.backupValue = iom.value
	iom.backupExtraData = iom.extraData
	iom.rebalanceStarted = true
	return true
}

func (iom *intOperationManager) EndRebalance() {
	if !iom.rebalanceStarted {
		panic("EndRebalance called when StartRebalance was not called or already ended")
	}
	iom.valueIndex = iom.backupIndex
	iom.value = iom.backupValue
	iom.extraData = iom.backupExtraData
	iom.rebalanceStarted = false
}

func (iom *intOperationManager) GetClone() OperationManager {
	clone := *iom
	return &clone
}

// AddToRebalanceList adds a node to a list of nodes to be rebalanced
func (iom *intOperationManager) AddToRebalanceList(node *binaryTreeConcurrentNode) {
	nodeLevel := node.level
	iom.st.rebalanceMutex.Lock()
	for len(iom.st.nodesToRebalance) <= nodeLevel {
		iom.st.nodesToRebalance = append(iom.st.nodesToRebalance, []*binaryTreeConcurrentNode{})
	}
	for _, existingNode := range iom.st.nodesToRebalance[nodeLevel] {
		if existingNode == node {
			iom.st.rebalanceMutex.Unlock()
			return
		}
	}
	iom.st.nodesToRebalance[nodeLevel] = append(iom.st.nodesToRebalance[nodeLevel], node)
	iom.st.rebalanceMutex.Unlock()
}

// GetNodeToRebalance returns next node in rebalance list, or nil if list is empty
func (iom *intOperationManager) GetNodeToRebalance() (node *binaryTreeConcurrentNode) {
	var sliceSize int
	if iom.rebalanceTorchID == 0 {
		iom.rebalanceTorchID = atomic.AddInt32(&iom.st.rebalanceTorch, 1)
	} else if iom.rebalanceTorchID != atomic.LoadInt32(&iom.st.rebalanceTorch) {
		return
	}
	iom.st.rebalanceMutex.RLock()
	for index, nodesToRebalance := range iom.st.nodesToRebalance {
		sliceSize = len(nodesToRebalance)
		if sliceSize > 0 {
			node = nodesToRebalance[sliceSize-1]
			iom.st.nodesToRebalance[index] = nodesToRebalance[:sliceSize-1]
			iom.st.rebalanceMutex.RUnlock()
			return
		}
	}
	iom.st.rebalanceMutex.RUnlock()
	return
}

// IntTree a binary tree of string values
type IntTree struct {
	rwmutex          sync.RWMutex
	ints             []int
	extraData        []interface{}
	freeList         []int
	rootNode         *binaryTreeConcurrentNode
	StoreExtraData   bool           // to be set if desired
	LogFunction      BtpLogFunction // to be set if desired
	OnRebalance      func()         // to be set if desired
	rebalanceMutex   sync.RWMutex
	nodesToRebalance [][]*binaryTreeConcurrentNode
	rebalanceTorch   int32
}

func (st *IntTree) getRootNode() *binaryTreeConcurrentNode {
	if st.rootNode == nil {
		if st.LogFunction == nil {
			st.LogFunction = func(stringToLog string) bool {
				return false
			}
		}
		if st.OnRebalance == nil {
			st.OnRebalance = func() {}
		}
		st.rootNode = &binaryTreeConcurrentNode{valueIndex: -1, branchBoundaries: [2]int{-1, -1}}
	}
	return st.rootNode
}

func (st *IntTree) SearchForValue(valueToFind int, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &intOperationManager{value: valueToFind, valueIndex: -1, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}, st: st}
	logFunction := btpSetLogFunction(st.LogFunction, logID)
	btpSearch(st.getRootNode(), onStart, logFunction, btom)
	nodeToRebalance := btom.GetNodeToRebalance()
	for nodeToRebalance != nil {
		btpRebalance(nodeToRebalance, logFunction, btom)
		nodeToRebalance = btom.GetNodeToRebalance()
	}
}

func (st *IntTree) InsertValue(valueToInsert int, extraData interface{}, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &intOperationManager{value: valueToInsert, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}
	logFunction := btpSetLogFunction(st.LogFunction, logID)
	btpInsert(st.getRootNode(), btom, onStart, logFunction)
	nodeToRebalance := btom.GetNodeToRebalance()
	for nodeToRebalance != nil {
		btpRebalance(nodeToRebalance, logFunction, btom)
		nodeToRebalance = btom.GetNodeToRebalance()
	}
}

func (st *IntTree) DeleteValue(valueToDelete int, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &intOperationManager{value: valueToDelete, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}
	logFunction := btpSetLogFunction(st.LogFunction, logID)
	btpDelete(st.getRootNode(), btom, onStart, false, false, logFunction)
	nodeToRebalance := btom.GetNodeToRebalance()
	for nodeToRebalance != nil {
		btpRebalance(nodeToRebalance, logFunction, btom)
		nodeToRebalance = btom.GetNodeToRebalance()
	}
}
