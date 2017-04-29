package binaryTree

import (
	"sync"
	"sync/atomic"
)

type stringOperationManager struct {
	value            string
	valueIndex       int
	extraData        interface{}
	st               *StringTree
	handleResult     func(int, bool)
	rebalanceTorchID int32
}

func (som *stringOperationManager) StoreValue() int {
	if som.valueIndex == -1 {
		som.st.rwmutex.Lock()
		freeListLength := len(som.st.freeList)
		if freeListLength > 1 {
			som.valueIndex = som.st.freeList[freeListLength-1]
			som.st.freeList = som.st.freeList[:freeListLength-1]
			som.st.strings[som.valueIndex] = som.value
			if som.st.StoreExtraData {
				som.st.extraData[som.valueIndex] = som.extraData
			}
		} else {
			som.valueIndex = len(som.st.strings)
			som.st.strings = append(som.st.strings, som.value)
			if som.st.StoreExtraData {
				if len(som.st.extraData) != som.valueIndex {
					panic("Size of extraData array should match size of String array.")
				}
				som.st.extraData = append(som.st.extraData, som.extraData)
			}
		}
		som.st.rwmutex.Unlock()
	}
	return som.valueIndex
}

func (som *stringOperationManager) DeleteValue() {
	if som.valueIndex > -1 {
		som.st.rwmutex.Lock()
		som.st.freeList = append(som.st.freeList, som.valueIndex)
		som.st.rwmutex.Unlock()
		som.valueIndex = -1
	}
}

func (som *stringOperationManager) CompareValueTo(valueIndex int) (comparisonResult int) {
	som.st.rwmutex.RLock()
	storedValue := som.st.strings[valueIndex]
	som.st.rwmutex.RUnlock()
	if som.value < storedValue {
		return -1
	} else if som.value > storedValue {
		return 1
	}
	if som.valueIndex < 0 {
		som.valueIndex = valueIndex
	} else if som.valueIndex != valueIndex {
		panic("value stored twice")
	}
	return 0
}

func (som *stringOperationManager) GetValueString() string {
	return som.value
}

func (som *stringOperationManager) GetStoredValueString(valueIndex int) string {
	if valueIndex < 0 {
		return "nil"
	}
	som.st.rwmutex.RLock()
	storedValue := som.st.strings[valueIndex]
	som.st.rwmutex.RUnlock()
	return storedValue
}

func (som *stringOperationManager) SetValueToStoredValue(valueIndex int) {
	som.st.rwmutex.RLock()
	som.value = som.st.strings[valueIndex]
	if som.st.StoreExtraData {
		som.extraData = som.st.extraData[valueIndex]
	}
	som.st.rwmutex.RUnlock()
	som.valueIndex = valueIndex
}

func (som *stringOperationManager) HandleResult(matchIndex int, matchFound bool) {
	if som.handleResult != nil {
		som.handleResult(matchIndex, matchFound)
		som.handleResult = nil
	}
}

func (som *stringOperationManager) OnRebalance() {
	som.st.OnRebalance()
}

// AddToRebalanceList adds a node to a list of nodes to be rebalanced
func (som *stringOperationManager) AddToRebalanceList(node *binaryTreeConcurrentNode) {
	som.st.btd.addToRebalanceList(node)
}

// GetNodeToRebalance returns next node in rebalance list, or nil if list is empty
func (som *stringOperationManager) GetNodeToRebalance() (node *binaryTreeConcurrentNode) {
	if som.rebalanceTorchID == 0 {
		som.rebalanceTorchID = atomic.AddInt32(&som.st.rebalanceTorch, 1)
	} else if som.rebalanceTorchID != atomic.LoadInt32(&som.st.rebalanceTorch) {
		return
	}
	return som.st.btd.getNodeToRebalance()
}

func (som *stringOperationManager) GetNewNode() *binaryTreeConcurrentNode {
	return som.st.btd.getNewNode()
}

// StringTree a binary tree of string values
type StringTree struct {
	rwmutex        sync.RWMutex
	strings        []string
	extraData      []interface{}
	freeList       []int
	rootNode       *binaryTreeConcurrentNode
	StoreExtraData bool           // to be set if desired
	LogFunction    BtpLogFunction // to be set if desired
	OnRebalance    func()         // to be set if desired
	rebalanceTorch int32
	btd            binaryTreeData
}

func (st *StringTree) getRootNode() *binaryTreeConcurrentNode {
	if st.rootNode == nil {
		if st.LogFunction == nil {
			st.LogFunction = func(stringToLog string) bool {
				return false
			}
		}
		if st.OnRebalance == nil {
			st.OnRebalance = func() {}
		}
		st.rootNode = st.btd.getNewNode()
	}
	return st.rootNode
}

func (st *StringTree) SearchForValue(valueToFind string, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &stringOperationManager{value: valueToFind, valueIndex: -1, handleResult: func(matchIndex int, matchFound bool) {
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
	st.btd.rebalanceNodes(logFunction, btom)
}

func (st *StringTree) InsertValue(valueToInsert string, extraData interface{}, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &stringOperationManager{value: valueToInsert, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
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
	st.btd.rebalanceNodes(logFunction, btom)
}

func (st *StringTree) DeleteValue(valueToDelete string, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btom := &stringOperationManager{value: valueToDelete, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
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
	st.btd.rebalanceNodes(logFunction, btom)
}
