package binaryTree

import (
	"sync"
)

type stringOperationManager struct {
	value        string
	valueIndex   int
	extraData    interface{}
	st           *StringTree
	handleResult func(int, bool)
}

func (som *stringOperationManager) StoreValue() int {
	if som.valueIndex == -1 {
		som.st.mutex.Lock()
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
		som.st.mutex.Unlock()
	}
	return som.valueIndex
}

func (som *stringOperationManager) UpdateValue(valueIndex int) {
	som.valueIndex = valueIndex
	som.st.mutex.Lock()
	som.st.strings[valueIndex] = som.value
	som.st.mutex.Unlock()
}

func (som *stringOperationManager) DeleteValue(valueIndex int) {
	som.st.mutex.Lock()
	som.st.freeList = append(som.st.freeList, valueIndex)
	som.st.mutex.Unlock()
}

func (som *stringOperationManager) CompareValueTo(valueIndex int) (comparisonResult int) {
	som.st.mutex.Lock()
	storedValue := som.st.strings[valueIndex]
	som.st.mutex.Unlock()
	if som.value < storedValue {
		return -1
	} else if som.value > storedValue {
		return 1
	}
	return 0
}

func (som *stringOperationManager) GetValueString() string {
	return som.value
}

func (som *stringOperationManager) GetStoredValueString(valueIndex int) string {
	som.st.mutex.Lock()
	storedValue := som.st.strings[valueIndex]
	som.st.mutex.Unlock()
	return storedValue
}

func (som *stringOperationManager) CloneWithStoredValue(valueIndex int) BinaryTreeOperationManager {
	som.st.mutex.Lock()
	storedValue := som.st.strings[valueIndex]
	som.st.mutex.Unlock()
	return &stringOperationManager{value: storedValue, valueIndex: valueIndex, handleResult: som.handleResult, st: som.st}
}

func (som *stringOperationManager) LaunchNewProcess(processFunction func()) {
	som.st.LaunchNewProcess(processFunction)
}

func (som *stringOperationManager) HandleResult(matchIndex int, matchFound bool) {
	som.handleResult(matchIndex, matchFound)
}

// StringTree a binary tree of string values
type StringTree struct {
	mutex            sync.Mutex
	strings          []string
	extraData        []interface{}
	freeList         []int
	rootNode         *binaryTreeConcurrentNode
	LaunchNewProcess func(func())   // to be set if desired
	StoreExtraData   bool           // to be set if desired
	LogFunction      BtpLogFunction // to be set if desired
	OnRebalance      func()         // to be set if desired
}

func (st *StringTree) getRootNode() *binaryTreeConcurrentNode {
	if st.rootNode == nil {
		if st.LaunchNewProcess == nil {
			st.LaunchNewProcess = func(processToLaunch func()) {
				go processToLaunch()
			}
		}
		if st.LogFunction == nil {
			st.LogFunction = func(getStringToLog func() string) bool {
				return false
			}
		}
		st.rootNode = &binaryTreeConcurrentNode{valueIndex: -1, branchBoundaries: [2]int{-1, -1}}
	}
	return st.rootNode
}

func (st *StringTree) SearchForValue(valueToFind string, resultHandler func(bool, interface{}), onStart func()) {
	btpSearch(st.getRootNode(), onStart, st.LogFunction, &stringOperationManager{value: valueToFind, valueIndex: -1, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}, st: st})
}

func (st *StringTree) InsertValue(valueToInsert string, extraData interface{}, resultHandler func(bool, interface{}), onStart func()) {
	btpInsert(st.getRootNode(), &stringOperationManager{value: valueToInsert, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}, onStart, st.OnRebalance, st.LogFunction)
}

func (st *StringTree) DeleteValue(valueToDelete string, resultHandler func(bool, interface{}), onStart func()) {
	btpDelete(st.getRootNode(), &stringOperationManager{value: valueToDelete, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}, onStart, st.OnRebalance, false, st.LogFunction)
}
