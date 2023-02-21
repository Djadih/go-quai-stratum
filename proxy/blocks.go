package proxy

import (
	"log"
	"math/big"
	// "strconv"
	// "strings"
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
	rpc := s.rpc()
	t := s.currentBlockTemplate()
	// pendingReply, height, _, err := s.fetchPendingBlock()
	// if err != nil {
	// 	log.Printf("Error while refreshing pending block on %s: %s", rpc.Name, err)
	// 	return
	// }
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

	// pendingReply.Difficulty = util.ToHex(s.config.Proxy.Difficulty)
	for _, s := range s.config.Proxy.Difficulty {
		log.Println(s)
	}
	// pendingReply.Difficulty = reply.DifficultyArray()
	// pendingReply.Difficulty = hexutil.EncodeBig(s.config.Proxy.Difficulty)

	// for i,s := range diff {

	// }

	newTemplate := BlockTemplate{
		Header:               pendingHeader,
		Target:               pendingHeader.DifficultyArray()[2],
		Height:               pendingHeader.NumberArray(),
		Difficulty:           pendingHeader.DifficultyArray()[2], //need to convert this with the formula
		// GetPendingBlockCache: pendingReply,
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
	log.Printf("New block to mine on %s at height %d / %s", rpc.Name, pendingHeader.NumberArray(), pendingHeader.NumberArray())

	// Stratum
	if s.config.Proxy.Stratum.Enabled {
		go s.broadcastNewJobs()
	}
}

/*
func (s *ProxyServer) fetchPendingBlock() (blockReply *rpc.GetBlockReplyPart, height []*big.Int, difficulty []*big.Int, err error) {
	rpc := s.rpc()
	reply, err := rpc.GetPendingBlock()
	if err != nil {
		log.Printf("Error while refreshing pending block on %s: %s", rpc.Name, err)
		return nil, nil, nil, err
	}
	var blockNumbers []*big.Int
	for _, blockNumStr := range reply.Number {
		blockNumber, err := hexutil.DecodeUint64(blockNumStr)
		if err != nil {
			log.Println("Can't parse hex block number.")
			return nil, nil, nil, err
		}

		log.Println("-------------------")
		log.Println(blockNumber)
		log.Println("-------------------")
		blockNumbers = append(blockNumbers, new(big.Int).SetUint64(blockNumber))

	}

	var blockDifficulties []*big.Int
	for _, blockDiffStr := range reply.Number {
		blockDiffNum, err := hexutil.DecodeUint64(blockDiffStr)
		if err != nil {
			log.Println("Can't parse hex block difficulty.")
			return nil, nil, nil, err
		}

		log.Println("-------------------")
		log.Println(blockDiffNum)
		log.Println("-------------------")
		blockDifficulties = append(blockDifficulties, new(big.Int).SetUint64(blockDiffNum))

	}

	// blockDiff, err := strconv.ParseInt(strings.Replace(reply.Difficulty, "0x", "", -1), 16, 64)
	// blockDiff := []string{}
	// var blockDiffs []int64
	// for _, s := range reply.Difficulty {
	// 	// blockDiff = append(blockDiff, )
	// 	num, err := strconv.ParseInt(strings.Replace(s, "0x", "", -1), 16, 64)
	// 	blockDiffs = append(blockDiffs, num)
	// 	if err != nil {
	// 		log.Println("Can't parse pending block difficulty")
	// 		return nil, nil, nil, err
	// 	}
	// }

	return reply, blockNumbers, blockDifficulties, nil
}
*/