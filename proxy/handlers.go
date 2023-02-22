package proxy

import (
	// "encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/INFURA/go-ethlibs/jsonrpc"

	"github.com/Djadih/go-quai-stratum/rpc"
	"github.com/Djadih/go-quai-stratum/util"
	"github.com/dominant-strategies/go-quai/core/types"
)

// Allow only lowercase hexadecimal with 0x prefix
var noncePattern = regexp.MustCompile("^0x[0-9a-f]{16}$")
var hashPattern = regexp.MustCompile("^0x[0-9a-f]{64}$")
var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_]{1,8}$")

// Stratum
func (s *ProxyServer) handleLoginRPC(cs *Session, params jsonrpc.Params) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}
	// var addy string
	// addy, err := jsonrpc.Unmarshal(params[0])
	// params[0].UnmarshalJSON([]byte(addy))
	// addy, err := jsonrpc.Unmarshal(params[0])
	addy, err := strconv.Unquote(string(params[0]))
	if err != nil {
		log.Printf("%v", err)
	}

	login := strings.ToLower(addy)
	if !util.IsValidHexAddress(login) {
		return false, &ErrorReply{Code: -1, Message: "Invalid login"}
	}
	if !s.policy.ApplyLoginPolicy(login, cs.ip) {
		return false, &ErrorReply{Code: -1, Message: "You are blacklisted"}
	}
	cs.login = login
	s.registerSession(cs)
	log.Printf("Stratum miner connected %v@%v", login, cs.ip)
	return true, nil
}

func (s *ProxyServer) handleGetWorkRPC(cs *Session) (*types.Header, *ErrorReply) {
	t := s.currentBlockTemplate()
	if t == nil || t.Header == nil || s.isSick() {
		return nil, &ErrorReply{Code: 0, Message: "Work not ready"}
	}
	// return []string{t.Header, t.Seed, s.diff}, nil
	return t.Header, nil
}

// Stratum
func (s *ProxyServer) handleTCPSubmitRPC(cs *Session, id string, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return s.handleSubmitRPC(cs, cs.login, id, params)
}

func (s *ProxyServer) handleSubmitRPC(cs *Session, login, id string, params []string) (bool, *ErrorReply) {
	if !workerPattern.MatchString(id) {
		id = "0"
	}
	if len(params) != 3 {
		s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed params from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	for i := 0; i <= 2; i++ {
		if params[i][0:2] != "0x" {
			params[i] = "0x" + params[i]
		}
	}

	if !noncePattern.MatchString(params[0]) || !hashPattern.MatchString(params[1]) || !hashPattern.MatchString(params[2]) {
		s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed PoW result from %s@%s %v", login, cs.ip, params)

		if !noncePattern.MatchString(params[0]) {
			log.Printf("[0] noncePattern %s", params[0])
		}
		if !hashPattern.MatchString(params[1]) {
			log.Printf("[1] hashPattern %s", params[1])
		}
		if !hashPattern.MatchString(params[2]) {
			log.Printf("[2] hashPattern %s", params[2])
		}

		return false, &ErrorReply{Code: -1, Message: "Malformed PoW result"}
	}
	/*
		t := s.currentBlockTemplate()


		exist, validShare := s.processShare(login, id, cs.ip, t, params)

		ok := s.policy.ApplySharePolicy(cs.ip, !exist && validShare)

		if exist {
			log.Printf("Duplicate share from %s@%s %v", login, cs.ip, params)
			return false, &ErrorReply{Code: 22, Message: "Duplicate share"}
			if !ok {
				return false, &ErrorReply{Code: 23, Message: "Invalid share"}
			}
			return false, nil
		}

		if !validShare {
			log.Printf("Invalid share from %s@%s", login, cs.ip)
			// Bad shares limit reached, return error and close
			if !ok {
				return false, &ErrorReply{Code: 23, Message: "Invalid share"}
			}
			return false, nil
		}
		log.Printf("Valid share from %s@%s", login, cs.ip)

		if !ok {
			return true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
		}
	*/
	return true, nil
}

func (s *ProxyServer) handleGetBlockByNumberRPC() *rpc.GetBlockReply {
	t := s.currentBlockTemplate()
	var reply *rpc.GetBlockReply
	if t != nil {
		reply = t.GetPendingBlockCache
	}
	return reply
}

func (s *ProxyServer) handleUnknownRPC(cs *Session, m string) *ErrorReply {
	log.Printf("Unknown request method %s from %s", m, cs.ip)
	s.policy.ApplyMalformedPolicy(cs.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}
