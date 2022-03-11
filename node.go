package art

import (
	"bytes"
	"math/bits"
)

func (l *leaf) prefixMatch(key Key) bool {
	if len(l.key) < len(key) {
		return false
	}

	return bytes.Compare(l.key[:len(key)], key) == 0
}

func (l *leaf) match(key Key) bool {
	if len(l.key) != len(key) {
		return false
	}
	return bytes.Compare(l.key, key) == 0
}

func (an *artNode) Type() NodeType {
	return an._type
}

func (an *artNode) Key() Key {
	if an.isLeaf() {
		return an.leaf().key
	}
	return nil
}

func (an *artNode) match(key Key, depth uint32) uint32 {
	idx := uint32(0)
	if len(key)-int(depth) < 0 {
		return idx
	}

	node := an.node()

	limit := min(min(node.prefixLen, MaxPrefixLen), uint32(len(key))-depth)
	for ; idx < limit; idx++ {
		if node.prefix[idx] != key[idx+depth] {
			return idx
		}
	}
	return idx
}

// find the minium leaf under a artNode
func (an *artNode) minimum() *leaf {

	switch an._type {
	case Leaf:
		return an.leaf()
	case Node4:
		node := an.node4()
		if node.zeroChild != nil {
			return node.zeroChild.minimum()
		} else if node.children[0] != nil {
			return node.children[0].minimum()
		}
	case Node16:
		node := an.node16()
		if node.zeroChild != nil {
			return node.zeroChild.minimum()
		} else if node.children[0] != nil {
			return node.children[0].minimum()
		}
	case Node48:
		node := an.node48()
		if node.zeroChild != nil {
			return node.zeroChild.minimum()
		}
		// find 1st child
		idx := uint8(0)
		for node.present[idx>>n48s]&(1<<uint8(idx%n48m)) == 0 {
			idx++
		}
		if node.children[node.keys[idx]] != nil {
			return node.children[node.keys[idx]].minimum()
		}
	case Node256:
		node := an.node256()
		if node.zeroChild != nil {
			return node.zeroChild.minimum()
		} else if len(node.children) > 0 {
			idx := 0
			for ; node.children[idx] == nil; idx++ {
				// find 1st non empty
			}
			return node.children[idx].minimum()
		}
	}
	return nil
}

// find mismatch index between key and leaf
func (an *artNode) matchDeep(key Key, depth uint32) uint32 {
	mismatchIdx := an.match(key, depth)
	if mismatchIdx < MaxPrefixLen {
		return mismatchIdx
	}
	leaf := an.minimum()
	limit := min(uint32(len(leaf.key)), uint32(len(key))) - depth
	for ; mismatchIdx < limit; mismatchIdx++ {
		if leaf.key[mismatchIdx+depth] != key[mismatchIdx+depth] {
			break
		}
	}

	return mismatchIdx
}

var nodeNotFound *artNode

func (an *artNode) findChild(c byte, valid bool) **artNode {
	node := an.node()

	if !valid {
		return &node.zeroChild
	}

	idx := an.index(c)
	if idx != -1 {
		switch an._type {
		case Node4:
			return &an.node4().children[idx]
		case Node16:
			return &an.node16().children[idx]
		case Node48:
			return &an.node48().children[idx]
		case Node256:
			return &an.node256().children[idx]
		}
	}

	return &nodeNotFound
}

func (an *artNode) index(c byte) int {
	switch an._type {
	case Node4:
		node := an.node4()
		for idx := 0; idx < int(node.numChildren); idx++ {
			if node.keys[idx] == c {
				return idx
			}
		}
	case Node16:
		node := an.node16()
		bitfield := uint(0)
		for i := uint(0); i < node16Max; i++ {
			if node.keys[i] == c {
				bitfield |= (1 << i)
			}
		}
		// if node.numChildren = 3, mask = 100
		mask := (1 << node.numChildren) - 1
		bitfield &= uint(mask)
		if bitfield != 0 {
			// get index of child ptr
			return bits.TrailingZeros(bitfield)
		}
	case Node48:
		node := an.node48()
		if s := node.present[c>>n48s] & (1 << (c % n48m)); s > 0 {
			if idx := int(node.keys[c]); idx >= 0 {
				return idx
			}
		}
	case Node256:
		return int(c)
	}
	return -1
}

func (an *artNode) addChild(c byte, valid bool, child *artNode) bool {
	switch an._type {
	case Node4:
		return an._addChild4(c, valid, child)
	case Node16:
		return an._addChild16(c, valid, child)
	case Node48:
		return an._addChild48(c, valid, child)
	case Node256:
		return an._addChild256(c, valid, child)
	}
	return false
}

func (an *artNode) _addChild4(c byte, valid bool, child *artNode) bool {
	node := an.node4()

	// grow to node16
	if node.numChildren >= node4Max {
		newNode := an.grow()
		newNode.addChild(c, valid, child)
		replaceNode(an, newNode)
		return true
	}

	// zero byte in the key
	if !valid {
		node.zeroChild = child
		return false
	}

	i := uint16(0)
	// maintain sorted order
	for ; i < node.numChildren; i++ {
		if c < node.keys[i] {
			break
		}
	}

	// shift right & insert key
	limit := node.numChildren - i
	for j := limit; limit > 0 && j > 0; j-- {
		// copy previous elem
		node.keys[i+j] = node.keys[i+j-1]
		node.present[i+j] = node.present[i+j-1]
		node.children[i+j] = node.children[i+j-1]
	}
	node.keys[i] = c
	node.present[i] = 1
	node.children[i] = child
	node.numChildren++

	return true
}

func (an *artNode) _addChild16(c byte, valid bool, child *artNode) bool {
	node := an.node16()

	if node.numChildren >= node16Max {
		newNode := an.grow()
		newNode.addChild(c, valid, child)
		replaceNode(an, newNode)
		return true
	}

	if !valid {
		node.zeroChild = child
		return false
	}

	idx := node.numChildren
	bitfield := uint(0)
	for i := uint(0); i < node16Max; i++ {
		if node.keys[i] > c {
			// mark index of key which is greater than c
			bitfield |= (i << i)
		}
	}

	mask := (1 << node.numChildren) - 1
	// clear last bit coz last bit of mask is 0
	bitfield &= uint(mask)
	if bitfield != 0 {
		idx = uint16(bits.TrailingZeros(bitfield))
	}
	// move all keys greater than c to right
	for i := node.numChildren; i > uint16(idx); i-- {
		node.keys[i] = node.keys[i-1]
		// The &^ operator is bit clear (AND NOT): in the expression z = x &^ y, each bit of z is 0 if the corresponding bit of y is 1;
		// otherwise it equals the corresponding bit of x.
		// (node.present & (1 << (i-1))) get the bit of i-1

		// this operation replace the bit of i with the bit of i-1
		node.present = (node.present & ^(1 << i)) | ((node.present & (1 << (i - 1))) << 1)
		node.children[i] = node.children[i-1]
	}

	node.keys[idx] = c
	node.present |= (1 << idx)
	node.children[idx] = child
	node.numChildren++
	return true
}
func (an *artNode) _addChild48(c byte, valid bool, child *artNode) bool {
	node := an.node48()

	if node.numChildren >= node16Max {
		newNode := an.grow()
		newNode.addChild(c, valid, child)
		replaceNode(an, newNode)
		return true
	}

	if !valid {
		node.zeroChild = child
		return false
	}
	index := byte(0)
	for node.children[index] != nil {
		index++
	}

	node.keys[c] = index
	node.present[c>>n48s] |= (1 << (c % n48m))
	node.children[index] = child
	node.numChildren++

	return true
}
func (an *artNode) _addChild256(c byte, valid bool, child *artNode) bool {
	node := an.node256()

	if !valid {
		node.zeroChild = child
	} else {
		node.numChildren++
		node.children[c] = child
	}

	return true
}

func (an *artNode) grow() *artNode {
	switch an._type {
	case Node4:
		// copy old node meta
		node := newNode16().copyMeta(an)

		d := node.node16()
		s := an.node4()
		d.zeroChild = s.zeroChild

		for i := 0; i < int(s.numChildren); i++ {
			if s.present[i] != 0 {
				d.keys[i] = s.keys[i]
				// set i'th bit for bitmap of node16
				d.present |= (1 << i)
				d.children[i] = s.children[i]
			}
		}
		return node
	case Node16:
		node := newNode48().copyMeta(an)

		d := node.node48()
		s := an.node16()

		d.zeroChild = s.zeroChild

		var numChildren byte
		for i := 0; i < int(s.numChildren); i++ {
			// check i'th bit
			if s.present&(1<<i) != 0 {
				ch := s.keys[i]
				// node48 map 256 keys, val of keys is index of child
				d.keys[ch] = numChildren
				// ch >> n48s == ch // 64
				d.present[ch>>n48s] |= 1 << (ch % n48m)
				d.children[numChildren] = s.children[i]
				numChildren++
			}
		}
		return node
	case Node48:
		node := newNode256().copyMeta(an)
		d := node.node256()
		s := an.node48()
		d.zeroChild = s.zeroChild
		for i := 0; i < node256Max; i++ {
			if s.present[i>>n48s]&(1<<(i%n48m)) != 0 {
				// get children index from val of key
				d.children[i] = s.children[s.keys[i]]
			}
		}
		return node
	}
	return nil
}

func (an *artNode) copyMeta(src *artNode) *artNode {
	if src == nil {
		return an
	}
	d := an.node()
	s := src.node()

	d.prefixLen = s.prefixLen
	d.numChildren = s.numChildren

	for i, limit := 0, min(s.prefixLen, MaxPrefixLen); i < int(limit); i++ {
		d.prefix[i] = s.prefix[i]
	}

	return an
}

func (an *artNode) node() *node {
	return (*node)(an.ref)
}

func (an *artNode) node4() *node4 {
	return (*node4)(an.ref)
}

func (an *artNode) node16() *node16 {
	return (*node16)(an.ref)
}

func (an *artNode) node48() *node48 {
	return (*node48)(an.ref)
}

func (an *artNode) node256() *node256 {
	return (*node256)(an.ref)
}

func (an *artNode) leaf() *leaf {
	return (*leaf)(an.ref)
}
func (an *artNode) setPrefix(key Key, prefixLen uint32) *artNode {
	nh := an.node()
	nh.prefixLen = prefixLen
	for i := uint32(0); i < min(prefixLen, MaxPrefixLen); i++ {
		nh.prefix[i] = key[i]
	}
	return an
}

func (an *artNode) isLeaf() bool {
	return an._type == Leaf
}

func longestCommonPrefix(l1, l2 *leaf, depth uint32) uint32 {
	idx, limit := depth, min(uint32(len(l1.key)), uint32(len(l2.key)))
	for ; idx < limit; idx++ {
		if l1.key[idx] != l2.key[idx] {
			break
		}
	}
	return idx - depth
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// modify oldNode ptr, ** means ref to pointer
func replaceRef(oldNode **artNode, newNode *artNode) {
	*oldNode = newNode
}

func replaceNode(oldNode *artNode, newNode *artNode) {
	*oldNode = *newNode
}
