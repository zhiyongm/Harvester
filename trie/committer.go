package trie

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type leaf struct {
	blob   []byte
	parent common.Hash
}

type committer struct {
	nodes       *NodeSet
	collectLeaf bool
}

func newCommitter(nodeset *NodeSet, collectLeaf bool) *committer {
	return &committer{
		nodes:       nodeset,
		collectLeaf: collectLeaf,
	}
}

func (c *committer) Commit(n node) hashNode {
	return c.commit(nil, n).(hashNode)
}

func (c *committer) commit(path []byte, n node) node {
	hash, dirty := n.cache()
	if hash != nil && !dirty {
		return hash
	}
	switch cn := n.(type) {
	case *shortNode:
		collapsed := cn.copy()

		if _, ok := cn.Val.(*fullNode); ok {
			collapsed.Val = c.commit(append(path, cn.Key...), cn.Val)
		}
		collapsed.Key = hexToCompact(cn.Key)
		hashedNode := c.store(path, collapsed)
		if hn, ok := hashedNode.(hashNode); ok {
			return hn
		}
		return collapsed
	case *fullNode:
		hashedKids := c.commitChildren(path, cn)
		collapsed := cn.copy()
		collapsed.Children = hashedKids

		hashedNode := c.store(path, collapsed)
		if hn, ok := hashedNode.(hashNode); ok {
			return hn
		}
		return collapsed
	case hashNode:
		return cn
	default:
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

func (c *committer) commitChildren(path []byte, n *fullNode) [17]node {
	var children [17]node
	for i := 0; i < 16; i++ {
		child := n.Children[i]
		if child == nil {
			continue
		}
		if hn, ok := child.(hashNode); ok {
			children[i] = hn
			continue
		}
		children[i] = c.commit(append(path, byte(i)), child)
	}
	if n.Children[16] != nil {
		children[16] = n.Children[16]
	}
	return children
}

func (c *committer) store(path []byte, n node) node {
	var hash, _ = n.cache()

	if hash == nil {
		if _, ok := c.nodes.accessList[string(path)]; ok {
			c.nodes.markDeleted(path)
		}
		return n
	}
	var (
		size  = estimateSize(n)
		nhash = common.BytesToHash(hash)
		mnode = &memoryNode{
			hash: nhash,
			node: simplifyNode(n),
			size: uint16(size),
		}
	)
	c.nodes.markUpdated(path, mnode)

	if c.collectLeaf {
		if sn, ok := n.(*shortNode); ok {
			if val, ok := sn.Val.(valueNode); ok {
				c.nodes.addLeaf(&leaf{blob: val, parent: nhash})
			}
		}
	}
	return hash
}

func estimateSize(n node) int {
	switch n := n.(type) {
	case *shortNode:
		return 3 + len(n.Key) + estimateSize(n.Val)
	case *fullNode:
		s := 3
		for i := 0; i < 16; i++ {
			if child := n.Children[i]; child != nil {
				s += estimateSize(child)
			} else {
				s++
			}
		}
		return s
	case valueNode:
		return 1 + len(n)
	case hashNode:
		return 1 + len(n)
	default:
		panic(fmt.Sprintf("node type %T", n))
	}
}
