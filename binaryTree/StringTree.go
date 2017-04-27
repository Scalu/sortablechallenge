package binaryTree

import (
	"sync"
)

type stringOperationManager struct {
	value            string
	valueIndex       int
	extraData        interface{}
	backupValue      string
	backupIndex      int
	backupExtraData  interface{}
	rebalanceStarted bool
	st               *StringTree
	handleResult     func(int, bool)
	nodesToRebalance [][]*binaryTreeConcurrentNode
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

func (som *stringOperationManager) RestoreValue() int {
	som.valueIndex = -1
	return som.StoreValue()
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

func (som *stringOperationManager) StartRebalance() bool {
	if som.rebalanceStarted {
		return false
	}
	som.backupIndex = som.valueIndex
	som.backupValue = som.value
	som.backupExtraData = som.extraData
	som.rebalanceStarted = true
	return true
}

func (som *stringOperationManager) EndRebalance() {
	if !som.rebalanceStarted {
		panic("EndRebalance called when StartRebalance was not called or already ended")
	}
	som.valueIndex = som.backupIndex
	som.value = som.backupValue
	som.extraData = som.backupExtraData
	som.rebalanceStarted = false
}

func (som *stringOperationManager) GetClone() OperationManager {
	clone := *som
	return &clone
}

// AddToRebalanceList adds a node to a list of nodes to be rebalanced
func (som *stringOperationManager) AddToRebalanceList(node *binaryTreeConcurrentNode) {
	nodeLevel := node.level
	for len(som.nodesToRebalance) <= nodeLevel {
		som.nodesToRebalance = append(som.nodesToRebalance, []*binaryTreeConcurrentNode{})
	}
	for _, existingNode := range som.nodesToRebalance[nodeLevel] {
		if existingNode == node {
			return
		}
	}
	som.nodesToRebalance[nodeLevel] = append(som.nodesToRebalance[nodeLevel], node)
}

// GetNodeToRebalance returns next node in rebalance list, or nil if list is empty
func (som *stringOperationManager) GetNodeToRebalance() (node *binaryTreeConcurrentNode) {
	var sliceSize int
	for index, nodesToRebalance := range som.nodesToRebalance {
		sliceSize = len(nodesToRebalance)
		if sliceSize > 0 {
			node = nodesToRebalance[sliceSize-1]
			som.nodesToRebalance[index] = nodesToRebalance[:sliceSize-1]
			return
		}
	}
	return
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
		st.rootNode = &binaryTreeConcurrentNode{valueIndex: -1, branchBoundaries: [2]int{-1, -1}}
	}
	return st.rootNode
}

func (st *StringTree) SearchForValue(valueToFind string, resultHandler func(bool, interface{}), onStart func(), logID string) {
	btpSearch(st.getRootNode(), onStart, btpSetLogFunction(st.LogFunction, logID), &stringOperationManager{value: valueToFind, valueIndex: -1, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}, st: st})
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
	nodeToRebalance := btom.GetNodeToRebalance()
	for nodeToRebalance != nil {
		btpRebalance(nodeToRebalance, logFunction, btom)
		nodeToRebalance = btom.GetNodeToRebalance()
	}
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
	nodeToRebalance := btom.GetNodeToRebalance()
	for nodeToRebalance != nil {
		btpRebalance(nodeToRebalance, logFunction, btom)
		nodeToRebalance = btom.GetNodeToRebalance()
	}
}
