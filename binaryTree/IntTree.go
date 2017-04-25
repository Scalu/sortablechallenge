package binaryTree

import (
	"fmt"
	"sync"
)

type intOperationManager struct {
	value        int
	valueIndex   int
	extraData    interface{}
	st           *IntTree
	handleResult func(int, bool)
}

func (som *intOperationManager) StoreValue() int {
	if som.valueIndex == -1 {
		som.st.mutex.Lock()
		freeListLength := len(som.st.freeList)
		if freeListLength > 1 {
			som.valueIndex = som.st.freeList[freeListLength-1]
			som.st.freeList = som.st.freeList[:freeListLength-1]
			som.st.ints[som.valueIndex] = som.value
			if som.st.StoreExtraData {
				som.st.extraData[som.valueIndex] = som.extraData
			}
		} else {
			som.valueIndex = len(som.st.ints)
			som.st.ints = append(som.st.ints, som.value)
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

func (som *intOperationManager) UpdateValue(valueIndex int) {
	som.valueIndex = valueIndex
	som.st.mutex.Lock()
	som.st.ints[valueIndex] = som.value
	som.st.mutex.Unlock()
}

func (som *intOperationManager) DeleteValue(valueIndex int) {
	som.st.mutex.Lock()
	som.st.freeList = append(som.st.freeList, valueIndex)
	som.st.mutex.Unlock()
}

func (som *intOperationManager) CompareValueTo(valueIndex int) (comparisonResult int) {
	som.st.mutex.Lock()
	storedValue := som.st.ints[valueIndex]
	som.st.mutex.Unlock()
	if som.value < storedValue {
		return -1
	} else if som.value > storedValue {
		return 1
	}
	return 0
}

func (som *intOperationManager) GetValueString() string {
	return fmt.Sprint(som.value)
}

func (som *intOperationManager) GetStoredValueString(valueIndex int) string {
	som.st.mutex.Lock()
	storedValue := som.st.ints[valueIndex]
	som.st.mutex.Unlock()
	return fmt.Sprint(storedValue)
}

func (som *intOperationManager) CloneWithStoredValue(valueIndex int) BinaryTreeOperationManager {
	som.st.mutex.Lock()
	storedValue := som.st.ints[valueIndex]
	som.st.mutex.Unlock()
	return &intOperationManager{value: storedValue, valueIndex: valueIndex, handleResult: som.handleResult, st: som.st}
}

func (som *intOperationManager) LaunchNewProcess(processFunction func()) {
	som.st.LaunchNewProcess(processFunction)
}

func (som *intOperationManager) HandleResult(matchIndex int, matchFound bool) {
	som.handleResult(matchIndex, matchFound)
}

// IntTree a binary tree of string values
type IntTree struct {
	mutex            sync.Mutex
	ints             []int
	extraData        []interface{}
	freeList         []int
	rootNode         *binaryTreeConcurrentNode
	LaunchNewProcess func(func())   // to be set if desired
	StoreExtraData   bool           // to be set if desired
	LogFunction      BtpLogFunction // to be set if desired
	OnRebalance      func()         // to be set if desired
}

func (st *IntTree) getRootNode() *binaryTreeConcurrentNode {
	if st.rootNode == nil {
		if st.LaunchNewProcess == nil {
			st.LaunchNewProcess = func(processToLaunch func()) {
				go processToLaunch()
			}
		}
		if st.LogFunction == nil {
			st.LogFunction = func(stringToLog string) bool {
				return false
			}
		}
		st.rootNode = &binaryTreeConcurrentNode{valueIndex: -1, branchBoundaries: [2]int{-1, -1}}
	}
	return st.rootNode
}

func (st *IntTree) SearchForValue(valueToFind int, resultHandler func(bool, interface{}), onStart func()) {
	btpSearch(st.getRootNode(), onStart, st.LogFunction, &intOperationManager{value: valueToFind, valueIndex: -1, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}, st: st})
}

func (st *IntTree) InsertValue(valueToInsert int, extraData interface{}, resultHandler func(bool, interface{}), onStart func()) {
	btpInsert(st.getRootNode(), &intOperationManager{value: valueToInsert, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}, onStart, st.OnRebalance, st.LogFunction)
}

func (st *IntTree) DeleteValue(valueToDelete int, resultHandler func(bool, interface{}), onStart func()) {
	btpDelete(st.getRootNode(), &intOperationManager{value: valueToDelete, valueIndex: -1, st: st, handleResult: func(matchIndex int, matchFound bool) {
		if resultHandler != nil {
			var extraData interface{}
			if st.StoreExtraData {
				extraData = st.extraData[matchIndex]
			}
			resultHandler(matchFound, extraData)
		}
	}}, onStart, st.OnRebalance, false, st.LogFunction)
}
