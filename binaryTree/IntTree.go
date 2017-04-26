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

func (iom *intOperationManager) StoreValue() int {
	if iom.valueIndex == -1 {
		iom.st.mutex.Lock()
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
		iom.st.mutex.Unlock()
	}
	return iom.valueIndex
}

func (iom *intOperationManager) RestoreValue() int {
	iom.valueIndex = -1
	return iom.StoreValue()
}

func (iom *intOperationManager) DeleteValue() {
	if iom.valueIndex > -1 {
		iom.st.mutex.Lock()
		iom.st.freeList = append(iom.st.freeList, iom.valueIndex)
		iom.st.mutex.Unlock()
		iom.valueIndex = -1
	}
}

func (iom *intOperationManager) CompareValueTo(valueIndex int) (comparisonResult int) {
	iom.st.mutex.Lock()
	storedValue := iom.st.ints[valueIndex]
	iom.st.mutex.Unlock()
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
	iom.st.mutex.Lock()
	storedValue := iom.st.ints[valueIndex]
	iom.st.mutex.Unlock()
	return fmt.Sprint(storedValue)
}

func (iom *intOperationManager) SetValueToStoredValue(valueIndex int) {
	iom.st.mutex.Lock()
	iom.value = iom.st.ints[valueIndex]
	iom.st.mutex.Unlock()
	iom.valueIndex = valueIndex
}

func (iom *intOperationManager) HandleResult(matchIndex int, matchFound bool) {
	if iom.handleResult != nil {
		iom.handleResult(matchIndex, matchFound)
		iom.handleResult = nil
	}
}

// IntTree a binary tree of string values
type IntTree struct {
	mutex          sync.Mutex
	ints           []int
	extraData      []interface{}
	freeList       []int
	rootNode       *binaryTreeConcurrentNode
	StoreExtraData bool           // to be set if desired
	LogFunction    BtpLogFunction // to be set if desired
	OnRebalance    func()         // to be set if desired
}

func (st *IntTree) getRootNode() *binaryTreeConcurrentNode {
	if st.rootNode == nil {
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
