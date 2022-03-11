package art

import (
	"errors"
	"unsafe"
)

const (
	Leaf NodeType = iota
	Node4
	Node16
	Node48
	Node256
)

const (
	traverseStop traverseAction = iota
	traverseContinue
)

const (
	// node constraints
	node4Min = 2
	node4Max = 4

	node16Min = node4Max + 1
	node16Max = 16

	node48Min = node16Max + 1
	node48Max = 48

	node256Min = node48Max + 1
	node256Max = 256

	// MaxPrefixLen is maximum prefix length for internal nodes.
	MaxPrefixLen = 10

	// Node with 48 children
	n48s = 6  // 2^n48s == n48m
	n48m = 64 // it should be sizeof(node48.present[0])

	nullIdx = -1
)

var (
	ErrNoMoreNodes = errors.New("There are no more nodes in the tree")
)

type (
	tree struct {
		size int
		root *artNode
	}

	NodeType int
	Key      []byte

	artNode struct {
		_type NodeType
		ref   unsafe.Pointer
	}

	// leaf node with variable key len
	leaf struct {
		key Key
	}
	prefix [MaxPrefixLen]byte
	// node header
	node struct {
		prefixLen   uint32
		prefix      prefix
		numChildren uint16
		// a key with the null suffix will be stored as zeroChild
		zeroChild *artNode
	}
	// node with 4 children
	node4 struct {
		node

		children [node4Max]*artNode
		keys     [node4Max]byte
		// bool in go also uses 1 byte
		present [node4Max]byte
	}
	node16 struct {
		node

		children [node16Max]*artNode
		keys     [node16Max]byte
		// bitmap present that if the key is present
		present uint16
	}
	/*
		As the number of entries in a node increases, searching the key array becomes expensive.
		Therefore, nodes with more than 16 pointers do not store the keys explicitly.
		Instead, a 256-element array is used, which can be indexed with key bytes directly.
		If a node has between 17 and 48 child pointers, this array stores indexes into a second array which contains up to 48 pointers.
		This indirection saves space in comparison to 256 pointers of 8 bytes, because the indexes only require 6 bits (we use 1 byte for simplicity).
	*/
	node48 struct {
		node

		children [node48Max]*artNode
		keys     [node256Max]byte
		// need 256 bits for keys
		present [4]uint64
	}
	node256 struct {
		node

		children [node256Max]*artNode
	}

	Callback func(n Node) bool

	traverseAction int

	iteratorLevel struct {
		node     *artNode
		childIdx int
	}

	iterator struct {
		tree       *tree
		nextNode   *artNode
		depthLevel int
		depth      []*iteratorLevel
	}
)

func newNode4() *artNode {
	return &artNode{
		_type: Node4,
		ref:   unsafe.Pointer(&node4{}),
	}
}

func newNode16() *artNode {
	return &artNode{
		_type: Node16,
		ref:   unsafe.Pointer(&node16{}),
	}
}

func newNode48() *artNode {
	return &artNode{
		_type: Node48,
		ref:   unsafe.Pointer(&node48{}),
	}
}

func newNode256() *artNode {
	return &artNode{
		_type: Node256,
		ref:   unsafe.Pointer(&node256{}),
	}
}

func newLeaf(key Key) *artNode {
	clonedKey := append([]byte(nil), key...)
	return &artNode{
		_type: Leaf,
		ref: unsafe.Pointer(&leaf{
			key: clonedKey,
		}),
	}
}

func (t NodeType) String() string {
	return []string{"Leaf", "Node4", "Node16", "Node48", "Node256"}[t]
}

func (k Key) charAt(pos int) byte {
	if pos < 0 || pos >= len(k) {
		return 0
	}
	return k[pos]
}

func (k Key) valid(pos int) bool {
	return pos >= 0 && pos < len(k)
}
