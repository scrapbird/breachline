package cache

// LRUList maintains cache eviction order
type LRUList struct {
	head  *LRUNode
	tail  *LRUNode
	nodes map[string]*LRUNode
	size  int
}

// LRUNode represents a node in the LRU list
type LRUNode struct {
	key        string
	prev, next *LRUNode
}

// NewLRUList creates a new LRU list
func NewLRUList() *LRUList {
	head := &LRUNode{}
	tail := &LRUNode{}
	head.next = tail
	tail.prev = head

	return &LRUList{
		head:  head,
		tail:  tail,
		nodes: make(map[string]*LRUNode),
	}
}

// AddToFront adds a key to the front of the LRU list
func (l *LRUList) AddToFront(key string) {
	if node, exists := l.nodes[key]; exists {
		l.moveToFront(node)
		return
	}

	node := &LRUNode{key: key}
	l.nodes[key] = node

	node.next = l.head.next
	node.prev = l.head
	l.head.next.prev = node
	l.head.next = node

	l.size++
}

// MoveToFront moves an existing key to the front
func (l *LRUList) MoveToFront(key string) {
	if node, exists := l.nodes[key]; exists {
		l.moveToFront(node)
	}
}

// Remove removes a key from the LRU list
func (l *LRUList) Remove(key string) {
	if node, exists := l.nodes[key]; exists {
		l.removeNode(node)
		delete(l.nodes, key)
		l.size--
	}
}

// RemoveOldest removes and returns the oldest key
func (l *LRUList) RemoveOldest() string {
	if l.size == 0 {
		return ""
	}

	oldest := l.tail.prev
	key := oldest.key
	l.removeNode(oldest)
	delete(l.nodes, key)
	l.size--

	return key
}

// Size returns the current size of the LRU list
func (l *LRUList) Size() int {
	return l.size
}

// moveToFront moves a node to the front of the list
func (l *LRUList) moveToFront(node *LRUNode) {
	l.removeNode(node)

	node.next = l.head.next
	node.prev = l.head
	l.head.next.prev = node
	l.head.next = node
}

// removeNode removes a node from the list
func (l *LRUList) removeNode(node *LRUNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}
