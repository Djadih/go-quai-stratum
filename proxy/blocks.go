package proxy

import (
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/dominant-strategies/go-quai/common"
	"github.com/dominant-strategies/go-quai/core/types"

	"github.com/Djadih/go-quai-stratum/rpc"
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
	Height               []uint64
	GetPendingBlockCache *rpc.GetBlockReplyPart
	nonces               map[string]bool
	headers              map[string]heightDiffPair
}

type Block struct {
	difficulty  []*hexutil.Big
	hashNoNonce common.Hash
	nonce       uint64
	number      uint64
}

func (b Block) Difficulty() []*hexutil.Big     { return b.difficulty }
func (b Block) HashNoNonce() common.Hash { return b.hashNoNonce }
func (b Block) Nonce() uint64            { return b.nonce }
func (b Block) NumberU64() uint64        { return b.number }

func (s *ProxyServer) fetchBlockTemplate() {
	rpc := s.rpc()
	t := s.currentBlockTemplate()
	pendingReply, height, diff, err := s.fetchPendingBlock()
	if err != nil {
		log.Printf("Error while refreshing pending block on %s: %s", rpc.Name, err)
		return
	}
	reply, err := rpc.GetWork()
	if err != nil {
		log.Printf("Error while refreshing block template on %s: %s", rpc.Name, err)
		return
	}
	// No need to update, we have fresh job
	if t != nil && t.Header == reply {
		return
	}

	// pendingReply.Difficulty = util.ToHex(s.config.Proxy.Difficulty)
	for _,s := range s.config.Proxy.Difficulty {
		log.Println(s)
	}
	pendingReply.Difficulty = []string{"5","6"}
	// pendingReply.Difficulty = hexutil.EncodeBig(s.config.Proxy.Difficulty)

	// for i,s := range diff {

	// }

	newTemplate := BlockTemplate{
		Header:               reply,
		Target:               reply.DifficultyArray()[2], //verify that zone difficulty comes first in the array
		Height:               height,
		Difficulty:           big.NewInt(diff[0]),
		GetPendingBlockCache: pendingReply,
		headers:              make(map[string]heightDiffPair),
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
	log.Printf("New block to mine on %s at height %d / %s", rpc.Name, height, reply.Number)

	// Stratum
	if s.config.Proxy.Stratum.Enabled {
		go s.broadcastNewJobs()
	}
}

func (s *ProxyServer) fetchPendingBlock() (*rpc.GetBlockReplyPart, []uint64, []int64, error) {
	rpc := s.rpc()
	reply, err := rpc.GetPendingBlock()
	if err != nil {
		log.Printf("Error while refreshing pending block on %s: %s", rpc.Name, err)
		return nil, nil, nil, err
	}
	var blockNumbers []uint64
	for _,s := range reply.Number {
		blockNumber, err := hexutil.DecodeUint64(s)
		if err != nil {
			log.Println("Can't parse hex block number.")
			return nil, nil, nil, err
		}
		// blockNumber, err := strconv.ParseUint(s, 16, 64)
		blockNumbers = append(blockNumbers, blockNumber)
		log.Println("-------------------")
		log.Println(blockNumber)
		log.Println("-------------------")
		if err != nil {
			log.Println("Can't parse pending block number")
			return nil, nil, nil, err
		}
	}

	// blockDiff, err := strconv.ParseInt(strings.Replace(reply.Difficulty, "0x", "", -1), 16, 64)
	// blockDiff := []string{}
	var blockDiffs []int64
	for _,s := range reply.Difficulty {
		// blockDiff = append(blockDiff, )
		num, err := strconv.ParseInt(strings.Replace(s, "0x", "", -1), 16, 64)
		blockDiffs = append(blockDiffs, num)
		if err != nil {
			log.Println("Can't parse pending block difficulty")
			return nil, nil, nil, err
		}
	}

	return reply, blockNumbers, blockDiffs, nil
}
