package binaryTree

import (
	"sync"
)

type rebalanceList struct {
	indices []*binaryTreeConcurrentNode
	mutex   sync.RWMutex
}

type binaryTreeData struct {
	nodeStorage      [][]binaryTreeConcurrentNode // contains node objects, allocated in groups of 256
	nodeStorageMutex sync.Mutex                   // the mutex used to lock the storage when creating new nodes
	nextNodeIndex    int32                        // the next index for when a new node is needed
	nodesToRebalance []*rebalanceList             // list of nodes to be rebalanced
	rebalanceMutex   sync.RWMutex                 // the mutex  used for locking the rebalance list
}

func (btd *binaryTreeData) getNewNode() *binaryTreeConcurrentNode {
	btd.nodeStorageMutex.Lock()
	newNodeIndex := btd.nextNodeIndex
	btd.nextNodeIndex++
	if newNodeIndex>>8 >= int32(len(btd.nodeStorage)) {
		btd.nodeStorage = append(btd.nodeStorage, make([]binaryTreeConcurrentNode, 256))
	}
	newNode := &btd.nodeStorage[newNodeIndex>>8][newNodeIndex&255]
	btd.nodeStorageMutex.Unlock()
	newNode.valueIndex = -1
	newNode.branchBoundaries = [2]int{-1, -1}
	return newNode
}

// AddToRebalanceList adds a node to a list of nodes to be rebalanced
func (btd *binaryTreeData) addToRebalanceList(node *binaryTreeConcurrentNode) {
	nodeLevel := node.level
	btd.rebalanceMutex.RLock()
	var levelList *rebalanceList
	if len(btd.nodesToRebalance) <= nodeLevel {
		btd.rebalanceMutex.RUnlock()
		btd.rebalanceMutex.Lock()
		for len(btd.nodesToRebalance) <= nodeLevel {
			btd.nodesToRebalance = append(btd.nodesToRebalance, &rebalanceList{indices: make([]*binaryTreeConcurrentNode, 0, 2)})
		}
		levelList = btd.nodesToRebalance[nodeLevel]
		btd.rebalanceMutex.Unlock()
	} else {
		levelList = btd.nodesToRebalance[nodeLevel]
		btd.rebalanceMutex.RUnlock()
	}
	levelList.mutex.Lock()
	levelList.indices = append(levelList.indices, node)
	levelList.mutex.Unlock()
}

// GetNodeToRebalance returns next node in rebalance list, or nil if list is empty
func (btd *binaryTreeData) getNodeToRebalance() (node *binaryTreeConcurrentNode) {
	var sliceSize int
	btd.rebalanceMutex.RLock()
	var levelList *rebalanceList
	for _, levelList = range btd.nodesToRebalance {
		levelList.mutex.Lock()
		sliceSize = len(levelList.indices)
		if sliceSize > 0 {
			node = levelList.indices[sliceSize-1]
			levelList.indices = levelList.indices[:sliceSize-1]
			levelList.mutex.Unlock()
			btd.rebalanceMutex.RUnlock()
			return
		}
		levelList.mutex.Unlock()
	}
	btd.rebalanceMutex.RUnlock()
	return
}

func (btd *binaryTreeData) rebalanceNodes(logFunction BtpLogFunction, btom OperationManager) {
	nodeToRebalance := btom.GetNodeToRebalance()
	for nodeToRebalance != nil {
		btpRebalance(nodeToRebalance, logFunction, btom)
		nodeToRebalance = btom.GetNodeToRebalance()
	}
}
