package trie

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type SecureTrie = StateTrie

func NewSecure(stateRoot common.Hash, owner common.Hash, root common.Hash, db *Database) (*SecureTrie, error) {
	id := &ID{
		StateRoot: stateRoot,
		Owner:     owner,
		Root:      root,
	}
	return NewStateTrie(id, db)
}

type StateTrie struct {
	trie             Trie
	preimages        *preimageStore
	hashKeyBuf       [common.HashLength]byte
	secKeyCache      map[string][]byte
	secKeyCacheOwner *StateTrie
}

func NewStateTrie(id *ID, db *Database) (*StateTrie, error) {
	if db == nil {
		panic("trie.NewStateTrie called without a database")
	}
	trie, err := New(id, db)
	if err != nil {
		return nil, err
	}
	return &StateTrie{trie: *trie, preimages: db.preimages}, nil
}

func (t *StateTrie) MustGet(key []byte) []byte {
	return t.trie.MustGet(t.hashKey(key))
}

func (t *StateTrie) GetStorage(_ common.Address, key []byte) ([]byte, error) {
	return t.trie.Get(t.hashKey(key))
}

func (t *StateTrie) GetAccount(address common.Address) (*types.StateAccount, error) {
	res, err := t.trie.Get(t.hashKey(address.Bytes()))
	if res == nil || err != nil {
		return nil, err
	}
	ret := new(types.StateAccount)
	err = rlp.DecodeBytes(res, ret)
	return ret, err
}

func (t *StateTrie) GetAccountByHash(addrHash common.Hash) (*types.StateAccount, error) {
	res, err := t.trie.Get(addrHash.Bytes())
	if res == nil || err != nil {
		return nil, err
	}
	ret := new(types.StateAccount)
	err = rlp.DecodeBytes(res, ret)
	return ret, err
}

func (t *StateTrie) GetNode(path []byte) ([]byte, int, error) {
	return t.trie.GetNode(path)
}

func (t *StateTrie) MustUpdate(key, value []byte) {
	hk := t.hashKey(key)
	t.trie.MustUpdate(hk, value)
	t.getSecKeyCache()[string(hk)] = common.CopyBytes(key)
}

func (t *StateTrie) UpdateStorage(_ common.Address, key, value []byte) error {
	hk := t.hashKey(key)
	err := t.trie.Update(hk, value)
	if err != nil {
		return err
	}
	t.getSecKeyCache()[string(hk)] = common.CopyBytes(key)
	return nil
}

func (t *StateTrie) UpdateAccount(address common.Address, acc *types.StateAccount) error {
	hk := t.hashKey(address.Bytes())
	data, err := rlp.EncodeToBytes(acc)
	if err != nil {
		return err
	}
	if err := t.trie.Update(hk, data); err != nil {
		return err
	}
	t.getSecKeyCache()[string(hk)] = address.Bytes()
	return nil
}

func (t *StateTrie) MustDelete(key []byte) {
	hk := t.hashKey(key)
	delete(t.getSecKeyCache(), string(hk))
	t.trie.MustDelete(hk)
}

func (t *StateTrie) DeleteStorage(_ common.Address, key []byte) error {
	hk := t.hashKey(key)
	delete(t.getSecKeyCache(), string(hk))
	return t.trie.Delete(hk)
}

func (t *StateTrie) DeleteAccount(address common.Address) error {
	hk := t.hashKey(address.Bytes())
	delete(t.getSecKeyCache(), string(hk))
	return t.trie.Delete(hk)
}

func (t *StateTrie) GetKey(shaKey []byte) []byte {
	if key, ok := t.getSecKeyCache()[string(shaKey)]; ok {
		return key
	}
	if t.preimages == nil {
		return nil
	}
	return t.preimages.preimage(common.BytesToHash(shaKey))
}

func (t *StateTrie) Commit(collectLeaf bool) (common.Hash, *NodeSet) {
	if len(t.getSecKeyCache()) > 0 {
		if t.preimages != nil {
			preimages := make(map[common.Hash][]byte)
			for hk, key := range t.secKeyCache {
				preimages[common.BytesToHash([]byte(hk))] = key
			}
			t.preimages.insertPreimage(preimages)
		}
		t.secKeyCache = make(map[string][]byte)
	}
	return t.trie.Commit(collectLeaf)
}

func (t *StateTrie) Hash() common.Hash {
	return t.trie.Hash()
}

func (t *StateTrie) Copy() *StateTrie {
	return &StateTrie{
		trie:        *t.trie.Copy(),
		preimages:   t.preimages,
		secKeyCache: t.secKeyCache,
	}
}

func (t *StateTrie) NodeIterator(start []byte) NodeIterator {
	return t.trie.NodeIterator(start)
}

func (t *StateTrie) hashKey(key []byte) []byte {
	h := newHasher(false)
	h.sha.Reset()
	h.sha.Write(key)
	h.sha.Read(t.hashKeyBuf[:])
	returnHasherToPool(h)
	return t.hashKeyBuf[:]
}

func (t *StateTrie) getSecKeyCache() map[string][]byte {
	if t != t.secKeyCacheOwner {
		t.secKeyCacheOwner = t
		t.secKeyCache = make(map[string][]byte)
	}
	return t.secKeyCache
}
