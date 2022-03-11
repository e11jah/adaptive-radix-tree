package art

type Tree interface {
	Insert(key Key) bool
	ForEachKeyPrefix(prefix Key) []string
	Iterator() Iterator
	Size() int
}

type Iterator interface {
	HasNext() bool
	Next() (Node, error)
}

type Node interface {
	Type() NodeType
	Key() Key
}

func New() Tree {
	return &tree{}
}
