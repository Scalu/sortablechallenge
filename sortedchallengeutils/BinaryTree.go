package sortedchallengeutils

// BinaryTreeComparer This interface provides a method for camparing 2 values
type BinaryTreeComparer interface {
	binaryTreeCompare(a, b int) int
	getInsertValue() int
	dumpNode(value, parentValue, leftValue, rightValue, weight int)
}

type binaryTreeNode struct {
	value     int
	weight    int
	parent    *binaryTreeNode
	leftright [2]*binaryTreeNode
}

// BinaryTree incomplete weight-balanced tree implementation
// does not have delete functionality since it's not required at the moment
// it can be implemented later
type BinaryTree struct {
	rootNode       *binaryTreeNode
	rebalanceList  []*binaryTreeNode
	nodeCount      int
	rebalanceCount int
}

func (bt *BinaryTree) getNewNode(comparer BinaryTreeComparer, parent *binaryTreeNode, value int) (newNode *binaryTreeNode) {
	if value < 0 {
		value = comparer.getInsertValue()
	}
	bt.nodeCount++
	return &binaryTreeNode{value: value, parent: parent}
}

func (bt *BinaryTree) rebalanceNode(comparer BinaryTreeComparer, node *binaryTreeNode) {
	if node.weight > -2 && node.weight < 2 {
		return
	}
	bt.rebalanceCount++
	if bt.nodeCount == 25 {
		println("break here")
	}

	newRootSide := 0           // side from which the new root node is being taken from
	newRootSideWeightMod := -1 // weight modifier for nodes between the current and new roots
	rootNewSide := 1           // side where the old root node is being moved to
	if node.weight > 0 {       // swap sides if the weight is positive
		newRootSide = 1
		newRootSideWeightMod = 1
		rootNewSide = 0
	}
	// get a pointer to the old root node ptr
	oldRootPtr := &bt.rootNode
	if node.parent != nil {
		if node.parent.leftright[0] == node {
			oldRootPtr = &node.parent.leftright[0]
		} else {
			oldRootPtr = &node.parent.leftright[1]
		}
	}
	// get a pointer to the new root's pointer in it's parent
	newRootPtr := &node.leftright[newRootSide]
	var newRootNode *binaryTreeNode
	for *newRootPtr != nil {
		newRootNode = *newRootPtr
		if newRootNode.leftright[rootNewSide] != nil {
			newRootNode.weight += newRootSideWeightMod // modify the weight to account for the new root node being removed
			if newRootNode.weight < -1 || newRootNode.weight > 1 {
				bt.rebalanceList = append(bt.rebalanceList, newRootNode)
			}
			newRootPtr = &newRootNode.leftright[rootNewSide]
		} else {
			break
		}
	}
	// get a pointer to the old root's new parent
	oldRootNewParentPtr := &node.leftright[rootNewSide]
	var oldRootNewParent *binaryTreeNode
	for *oldRootNewParentPtr != nil {
		oldRootNewParent = *oldRootNewParentPtr
		oldRootNewParent.weight += newRootSideWeightMod // adjust the weight to account for the old root being added
		if oldRootNewParent.weight < -1 || oldRootNewParent.weight > 1 {
			bt.rebalanceList = append(bt.rebalanceList, oldRootNewParent)
		}
		if oldRootNewParent.leftright[newRootSide] != nil {
			oldRootNewParentPtr = &oldRootNewParent.leftright[newRootSide]
		} else {
			break
		}
	}
	// put the new root in the old root's place
	*oldRootPtr = newRootNode
	// cut out the new root if required
	if newRootNode.parent != node {
		*newRootPtr = newRootNode.leftright[newRootSide]
		if *newRootPtr != nil {
			(*newRootPtr).parent = newRootNode.parent
		}
		newRootNode.leftright[newRootSide] = node.leftright[newRootSide]
		node.leftright[newRootSide].parent = newRootNode
	}
	newRootNode.parent = node.parent
	newRootNode.weight = node.weight - 2*newRootSideWeightMod
	// check where the old root node goes
	if *oldRootNewParentPtr != nil {
		// new parent found for it
		newRootNode.leftright[rootNewSide] = node.leftright[rootNewSide]
		node.leftright[rootNewSide].parent = newRootNode
		node.parent = oldRootNewParent
		oldRootNewParent.leftright[newRootSide] = node
	} else {
		// new root node becomes it's parent
		newRootNode.leftright[rootNewSide] = node
		node.parent = newRootNode
	}
	// old root should have no children
	node.leftright[0] = nil
	node.leftright[1] = nil
	node.weight = 0
	// fmt.Println("rebalance done for node", node.value)
	// bt.DumpTree(comparer)
	return
}

func (bt *BinaryTree) processRebalanceList(comparer BinaryTreeComparer) {
	for len(bt.rebalanceList) > 0 {
		rebalanceListIndex := len(bt.rebalanceList) - 1
		nodeToRebalance := bt.rebalanceList[rebalanceListIndex]
		bt.rebalanceList = bt.rebalanceList[:rebalanceListIndex]
		bt.rebalanceNode(comparer, nodeToRebalance)
	}
	bt.rebalanceList = []*binaryTreeNode{}
}

// Insert inserts a value, and returns the actual value
func (bt *BinaryTree) Insert(comparer BinaryTreeComparer, value int) (actualValue int, valueAlreadyExists bool) {
	if bt.rootNode == nil {
		bt.rootNode = bt.getNewNode(comparer, nil, value)
		return bt.rootNode.value, false
	}
	defer bt.processRebalanceList(comparer)
	node := bt.rootNode
	var nextNodePtr **binaryTreeNode
	for {
		comparisonResult := comparer.binaryTreeCompare(node.value, value)
		if comparisonResult == 0 {
			actualValue = node.value
			// undo weight changes
			for node.parent != nil {
				if node.parent.leftright[0] == node {
					node.parent.weight++
				} else {
					node.parent.weight--
				}
				node = node.parent
			}
			return actualValue, true
		}
		if comparisonResult < 0 {
			node.weight-- // can't modify the weight if a value already exists
			nextNodePtr = &node.leftright[0]
		} else {
			node.weight++
			nextNodePtr = &node.leftright[1]
		}
		if node.weight < -1 || node.weight > 1 {
			bt.rebalanceList = append(bt.rebalanceList, node)
		}
		if *nextNodePtr == nil {
			*nextNodePtr = bt.getNewNode(comparer, node, value)
			// fmt.Println("Insert done for node value", (*nextNodePtr).value)
			// bt.DumpTree(comparer)
			return (*nextNodePtr).value, false
		}
		node = *nextNodePtr
	}
}

// Search searches for a value and return matching value
// returns -1 if there is no match
func (bt *BinaryTree) Search(comparer BinaryTreeComparer, searchValue int) (value int) {
	return -1
}

// DumpTree dumps the tree for debugging purposes
func (bt *BinaryTree) DumpTree(comparer BinaryTreeComparer) {
	nodeArray := []*binaryTreeNode{bt.rootNode}
	nodeIndex := 0
	for nodeIndex < len(nodeArray) {
		node := nodeArray[nodeIndex]
		nodeIndex++
		parentValue := -1
		if node.parent != nil {
			parentValue = node.parent.value
		}
		leftValue := -1
		if node.leftright[0] != nil {
			leftValue = node.leftright[0].value
			nodeArray = append(nodeArray, node.leftright[0])
		}
		rightValue := -1
		if node.leftright[1] != nil {
			rightValue = node.leftright[1].value
			nodeArray = append(nodeArray, node.leftright[1])
		}
		comparer.dumpNode(node.value, parentValue, leftValue, rightValue, node.weight)
	}
}
