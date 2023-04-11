package proxy

import (
	// "errors"
	"log"
	"math/big"

	"sync"

	"github.com/dominant-strategies/go-quai/common"
	"github.com/dominant-strategies/go-quai/consensus/blake3pow"
	"github.com/dominant-strategies/go-quai/core/types"

	"github.com/dominant-strategies/go-quai/common/hexutil"
)

const maxBacklog = 3

type BlockTemplate struct {
	sync.RWMutex
	Header     *types.Header
	Target     *big.Int
	Difficulty *big.Int
	Height     []*big.Int
}

type Block struct {
	difficulty  []*hexutil.Big
	hashNoNonce common.Hash
	nonce       uint64
	number      uint64
}

func (b Block) Difficulty() []*hexutil.Big { return b.difficulty }
func (b Block) HashNoNonce() common.Hash   { return b.hashNoNonce }
func (b Block) Nonce() uint64              { return b.nonce }
func (b Block) NumberU64() uint64          { return b.number }

func (s *ProxyServer) fetchBlockTemplate() {
	rpc := s.rpc(common.ZONE_CTX)
	t := s.currentBlockTemplate()
	pendingHeader, err := rpc.GetWork()
	if err != nil {
		log.Printf("Error while getting pending header (work) on %s: %s", rpc.Name, err)
		return
	}

	// Only update if the pending header has changed
	// if t != nil && t.Header != nil && t.Header.SealHash() == pendingHeader.SealHash() {
	if t != nil && t.Header != nil && t.Header.SealHash() == pendingHeader.SealHash() {
		return
	} else if t != nil {
		t.Header = pendingHeader
	}

	// tempTarget, suc := new(big.Int).SetString("0x000000FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", 0)
	// if !suc {
	// 	log.Printf("Error while converting target to big.Int")
	// 	panic(1)
	// }

	newTemplate := BlockTemplate{
		Header: pendingHeader,
		Target: blake3pow.DifficultyToTarget(pendingHeader.Difficulty()),
		// Target: tempTarget,
		Height: pendingHeader.NumberArray(),
	}

	s.blockTemplate.Store(&newTemplate)
	log.Printf("New block to mine on %s at height %d", rpc.Name, pendingHeader.NumberArray())

	if s.config.Proxy.Stratum.Enabled {
		go s.broadcastNewJobs()
	}
}
