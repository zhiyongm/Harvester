package trie

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

type memoryNode struct {
	hash common.Hash
	size uint16
	node node
}

var memoryNodeSize = int(reflect.TypeOf(memoryNode{}).Size())

func (n *memoryNode) memorySize(pathlen int) int {
	return int(n.size) + memoryNodeSize + pathlen
}

func (n *memoryNode) rlp() []byte {
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	return nodeToBytes(n.node)
}

func (n *memoryNode) obj() node {
	if node, ok := n.node.(rawNode); ok {
		return mustDecodeNode(n.hash[:], node)
	}
	return expandNode(n.hash[:], n.node)
}

func (n *memoryNode) isDeleted() bool {
	return n.hash == (common.Hash{})
}

type nodeWithPrev struct {
	*memoryNode
	prev []byte
}

func (n *nodeWithPrev) unwrap() *memoryNode {
	return n.memoryNode
}

func (n *nodeWithPrev) memorySize(pathlen int) int {
	return n.memoryNode.memorySize(pathlen) + len(n.prev)
}

type NodeSet struct {
	owner   common.Hash
	nodes   map[string]*memoryNode
	leaves  []*leaf
	updates int
	deletes int

	accessList map[string][]byte
}

func NewNodeSet(owner common.Hash, accessList map[string][]byte) *NodeSet {
	return &NodeSet{
		owner:      owner,
		nodes:      make(map[string]*memoryNode),
		accessList: accessList,
	}
}

func (set *NodeSet) forEachWithOrder(callback func(path string, n *memoryNode)) {
	var paths sort.StringSlice
	for path := range set.nodes {
		paths = append(paths, path)
	}
	sort.Sort(sort.Reverse(paths))
	for _, path := range paths {
		callback(path, set.nodes[path])
	}
}

func (set *NodeSet) markUpdated(path []byte, node *memoryNode) {
	set.nodes[string(path)] = node
	set.updates += 1
}

func (set *NodeSet) markDeleted(path []byte) {
	set.nodes[string(path)] = &memoryNode{}
	set.deletes += 1
}

func (set *NodeSet) addLeaf(node *leaf) {
	set.leaves = append(set.leaves, node)
}

func (set *NodeSet) Size() (int, int) {
	return set.updates, set.deletes
}

func (set *NodeSet) Hashes() []common.Hash {
	var ret []common.Hash
	for _, node := range set.nodes {
		ret = append(ret, node.hash)
	}
	return ret
}

func (set *NodeSet) Summary() string {
	var out = new(strings.Builder)
	fmt.Fprintf(out, "nodeset owner: %v\n", set.owner)
	if set.nodes != nil {
		for path, n := range set.nodes {
			if n.isDeleted() {
				fmt.Fprintf(out, "  [-]: %x prev: %x\n", path, set.accessList[path])
				continue
			}
			origin, ok := set.accessList[path]
			if !ok {
				fmt.Fprintf(out, "  [+]: %x -> %v\n", path, n.hash)
				continue
			}
			fmt.Fprintf(out, "  [*]: %x -> %v prev: %x\n", path, n.hash, origin)
		}
	}
	for _, n := range set.leaves {
		fmt.Fprintf(out, "[leaf]: %v\n", n)
	}
	return out.String()
}

type MergedNodeSet struct {
	sets map[common.Hash]*NodeSet
}

func NewMergedNodeSet() *MergedNodeSet {
	return &MergedNodeSet{sets: make(map[common.Hash]*NodeSet)}
}

func NewWithNodeSet(set *NodeSet) *MergedNodeSet {
	merged := NewMergedNodeSet()
	merged.Merge(set)
	return merged
}

func (set *MergedNodeSet) Merge(other *NodeSet) error {
	_, present := set.sets[other.owner]
	if present {
		return fmt.Errorf("duplicate trie for owner %#x", other.owner)
	}
	set.sets[other.owner] = other
	return nil
}
