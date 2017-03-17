package sortedchallengeutils

// RedBlackTreeComparer This interface provides a method for camparing 2 values
type RedBlackTreeComparer interface {
	redBlackTreeCompare(a, b int) int
	getInsertValue() int
}

const (
	red   = iota
	black = iota
)

type redBlackTreeNode struct {
	value    int
	redBlack int
	parent   int
	left     int
	right    int
}

// RedBlackTree incomplete red-black tree implementation
// does not have delete functionality since it's not required at the moment
// it can be implemented later
type RedBlackTree struct {
	nodes []redBlackTreeNode
}

// Insert inserts a value
func (rbt RedBlackTree) Insert(comparer RedBlackTreeComparer, value int) (index int, valueAlreadyExists bool) {
	return -1, true
}

// Search searches for a value and return matching value
// returns -1 if there is no match
func (rbt RedBlackTree) Search(comparer RedBlackTreeComparer, value int) (index int) {
	return -1
}
