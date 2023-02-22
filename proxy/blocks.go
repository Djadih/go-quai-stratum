package proxy

import (
	"errors"
	"log"
	"math/big"

	"sync"

	"github.com/dominant-strategies/go-quai/common"
	"github.com/dominant-strategies/go-quai/core/types"

	"github.com/dominant-strategies/go-quai-stratum/rpc"
	"github.com/dominant-strategies/go-quai/common/hexutil"
)

const maxBacklog = 3

type heightDiffPair struct {
	diff   *big.Int
	height []uint64
}

type BlockTemplate struct {
	sync.RWMutex
	Header               *types.Header
	Target               *big.Int
	Difficulty           *big.Int
	Height               []*big.Int
	GetPendingBlockCache *rpc.GetBlockReply
	nonces               map[string]bool
	headers              map[string]heightDiffPair
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
	// No need to update, we have fresh job
	if t != nil && t.Header == pendingHeader {
		return
	} else if t != nil {
		t.Header = pendingHeader
	}

	newTemplate := BlockTemplate{
		Header:     pendingHeader,
		Target:     pendingHeader.DifficultyArray()[2],
		Height:     pendingHeader.NumberArray(),
		Difficulty: pendingHeader.DifficultyArray()[2],
		// GetPendingBlockCache: pendingReply,
		headers: make(map[string]heightDiffPair),
	}

	// Needs to be replaced
	/*
		// Copy job backlog and add current one
		newTemplate.headers[reply[0]] = heightDiffPair{
			diff:   util.TargetHexToDiff(reply[2]),
			height: height,
		}
		if t != nil {
			for k, v := range t.headers {
				if v.height > height-maxBacklog {
					newTemplate.headers[k] = v
				}
			}
		}
	*/
	s.blockTemplate.Store(&newTemplate)
	log.Printf("New block to mine on %s at height %d / %s", rpc.Name, pendingHeader.NumberArray(), pendingHeader.NumberArray())

	if s.config.Proxy.Stratum.Enabled {
		go s.broadcastNewJobs()
	}
}

var (
	big2e256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0)) // 2^256
)

// This function determines the difficulty order of a block
func GetDifficultyOrder(header *types.Header) (int, error) {
	if header == nil {
		return common.HierarchyDepth, errors.New("no header provided")
	}
	blockhash := header.Hash()
	for i, difficulty := range header.DifficultyArray() {
		if difficulty != nil && big.NewInt(0).Cmp(difficulty) < 0 {
			target := new(big.Int).Div(big2e256, difficulty)
			if new(big.Int).SetBytes(blockhash.Bytes()).Cmp(target) <= 0 {
				return i, nil
			}
		}
	}
	return -1, errors.New("block does not satisfy minimum difficulty")
}
