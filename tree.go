package art

func (t *tree) Size() int {
	if t == nil || t.root == nil {
		return 0
	}
	return t.size
}

func (t *tree) Insert(key Key) bool {
	updated := t.recursiveInsert(&t.root, key, 0)
	if !updated {
		t.size++
	}
	return updated
}

func (t *tree) recursiveInsert(curNode **artNode, key Key, depth uint32) bool {
	curr := *curNode
	if curr == nil {
		replaceRef(curNode, newLeaf(key))
		return true
	}

	if curr.isLeaf() {
		leaf := curr.leaf()

		if leaf.match(key) {
			return true
		}
		// splilt leaf into new node4
		newLeaf := newLeaf(key)
		leaf2 := newLeaf.leaf()
		leafsLcp := longestCommonPrefix(leaf, leaf2, depth)

		newNode := newNode4()
		newNode.setPrefix(key[depth:], leafsLcp)
		depth += leafsLcp

		newNode.addChild(leaf.key.charAt(int(depth)), leaf.key.valid(int(depth)), curr)
		newNode.addChild(leaf2.key.charAt(int(depth)), leaf2.key.valid(int(depth)), newLeaf)
		replaceRef(curNode, newNode)

		return true
	}

	node := curr.node()
	if node.prefixLen > 0 {
		prefixMismatchIdx := curr.matchDeep(key, depth)

		if prefixMismatchIdx >= node.prefixLen {
			depth += node.prefixLen
			goto NEXT_NODE
		}

		// new node as parent
		newNode := newNode4()
		node4 := newNode.node()
		node4.prefixLen = prefixMismatchIdx
		for i := 0; i < int(min(prefixMismatchIdx, MaxPrefixLen)); i++ {
			node4.prefix[i] = node.prefix[i]
		}

		if node.prefixLen <= MaxPrefixLen {
			node.prefixLen -= (prefixMismatchIdx + 1)
			newNode.addChild(node.prefix[prefixMismatchIdx], true, curr)

			for i, limit := uint32(0), min(node.prefixLen, MaxPrefixLen); i < limit; i++ {
				node.prefix[i] = node.prefix[prefixMismatchIdx+i+1]
			}
		} else {
			// if prefix of node too long, get the prefix from key of first leaf
			node.prefixLen -= (prefixMismatchIdx + 1)
			leaf := curr.minimum()
			newNode.addChild(leaf.key.charAt(int(depth+prefixMismatchIdx)), leaf.key.valid(int(depth+prefixMismatchIdx)), curr)

			for i, limit := uint32(0), min(node.prefixLen, MaxPrefixLen); i < limit; i++ {
				node.prefix[i] = leaf.key[depth+prefixMismatchIdx+i+1]
			}
		}

		newNode.addChild(key.charAt(int(depth+prefixMismatchIdx)), key.valid(int(depth+prefixMismatchIdx)), newLeaf(key))
		replaceRef(curNode, newNode)
		return true
	}

NEXT_NODE:
	next := curr.findChild(key.charAt(int(depth)), key.valid(int(depth)))
	if *next != nil {
		return t.recursiveInsert(next, key, depth+1)
	}
	// no child found, create new leaf
	curr.addChild(key.charAt(int(depth)), key.valid(int(depth)), newLeaf(key))

	return true
}

func (t *tree) ForEachKeyPrefix(prefix Key) []Key {
	keys := make([]Key, 0)
	t.forEachPrefix(t.root, prefix, func(n Node) bool {
		if n.Type() != Leaf {
			return true
		}
		keys = append(keys, n.Key())
		return true
	})
	return keys
}

func (t *tree) forEachPrefix(curr *artNode, key Key, callback Callback) traverseAction {
	if curr == nil {
		return traverseContinue
	}

	depth := uint32(0)

	for curr != nil {
		if curr.isLeaf() {
			leaf := curr.leaf()
			if leaf.prefixMatch(key) {
				if !callback(curr) {
					return traverseStop
				}
			}
			break
		}

		if depth == uint32(len(key)) {
			leaf := curr.minimum()
			if leaf.prefixMatch(key) {
				if t.recursiveForEach(curr, callback) == traverseStop {
					return traverseStop
				}
			}
			break
		}

		node := curr.node()
		if node.prefixLen > 0 {
			prefixLen := curr.matchDeep(key, depth)
			if prefixLen > node.prefixLen {
				prefixLen = node.prefixLen
			}

			if prefixLen == 0 {
				break
			} else if depth+prefixLen == uint32(len(key)) {
				return t.recursiveForEach(curr, callback)
			}
			depth += node.prefixLen
		}

		next := curr.findChild(key.charAt(int(depth)), key.valid(int(depth)))
		if *next == nil {
			break
		}
		curr = *next
		depth++
	}

	return traverseContinue
}

func (t *tree) recursiveForEach(curr *artNode, callback Callback) traverseAction {
	if curr == nil {
		return traverseContinue
	}

	if !callback(curr) {
		return traverseStop
	}

	switch curr._type {
	case Node4:
		return t.forEachChildren(curr.node().zeroChild, curr.node4().children[:], callback)

	case Node16:
		return t.forEachChildren(curr.node().zeroChild, curr.node16().children[:], callback)
	case Node48:
		node := curr.node48()
		child := node.zeroChild
		if child != nil {
			if t.recursiveForEach(child, callback) == traverseStop {
				return traverseStop
			}
		}

		for i, c := range node.keys {
			if node.present[uint16(i)>>n48s]&(1<<(uint16(i)%n48m)) == 0 {
				continue
			}

			child := node.children[c]
			if child != nil {
				if t.recursiveForEach(child, callback) == traverseStop {
					return traverseStop
				}
			}
		}

	case Node256:
		return t.forEachChildren(curr.node().zeroChild, curr.node256().children[:], callback)
	}

	return traverseContinue
}

func (t *tree) forEachChildren(nullChild *artNode, children []*artNode, callback Callback) traverseAction {
	if nullChild != nil {
		if t.recursiveForEach(nullChild, callback) == traverseStop {
			return traverseStop
		}
	}

	for _, child := range children {
		if child != nil && child != nullChild {
			if t.recursiveForEach(child, callback) == traverseStop {
				return traverseStop
			}
		}
	}
	return traverseContinue
}

func (t *tree) Iterator() Iterator {
	return &iterator{
		tree:       t,
		nextNode:   t.root,
		depthLevel: 0,
		depth:      []*iteratorLevel{{t.root, nullIdx}},
	}
}

func (it *iterator) HasNext() bool {
	return it != nil && it.nextNode != nil
}

func (it *iterator) Next() (Node, error) {
	if !it.HasNext() {
		return nil, ErrNoMoreNodes
	}
	cur := it.nextNode
	it.next()
	return cur, nil
}

func (it *iterator) next() {
	var nextNode *artNode
	for {
		nextChildIdx := nullIdx

		curNode := it.depth[it.depthLevel].node
		curChildIdx := it.depth[it.depthLevel].childIdx

		switch curNode._type {
		case Node4:
			nextChildIdx, nextNode = nextChild(curChildIdx, curNode.node().zeroChild, curNode.node4().children[:])
		case Node16:
			nextChildIdx, nextNode = nextChild(curChildIdx, curNode.node().zeroChild, curNode.node16().children[:])
		case Node48:
			node := curNode.node48()
			nullChild := node.zeroChild
			if curChildIdx == nullIdx {
				if nullChild == nil {
					curChildIdx = 0
				} else {
					nextChildIdx = 0
					nextNode = nullChild
					break
				}
			}

			for i := curChildIdx; i < len(node.keys); i++ {
				if node.present[uint16(i)>>n48s]&(1<<(uint16(i)%n48m)) == 0 {
					continue
				}

				child := node.children[node.keys[i]]
				if child != nil && child != nullChild {
					nextChildIdx = i + 1
					nextNode = child
					break
				}
			}
		case Node256:
			nextChildIdx, nextNode = nextChild(curChildIdx, curNode.node().zeroChild, curNode.node256().children[:])
		}

		if nextNode == nil {
			if it.depthLevel > 0 {
				it.depthLevel--
			} else {
				it.nextNode = nil
				return
			}
		} else {
			it.depth[it.depthLevel].childIdx = nextChildIdx
			it.nextNode = nextNode

			if it.depthLevel+1 >= cap(it.depth) {
				newDepthLevel := make([]*iteratorLevel, it.depthLevel+2)
				copy(newDepthLevel, it.depth)
				it.depth = newDepthLevel
			}

			it.depthLevel++
			it.depth[it.depthLevel] = &iteratorLevel{nextNode, nullIdx}
			return
		}
	}
}

func nextChild(childIdx int, nullChild *artNode, children []*artNode) (int, *artNode) {
	if childIdx == nullIdx {
		if nullChild != nil {
			return 0, nullChild
		}
		childIdx = 0
	}

	for i := childIdx; i < len(children); i++ {
		child := children[i]
		if child != nil && child != nullChild {
			return i + 1, child
		}
	}

	return 0, nil
}
