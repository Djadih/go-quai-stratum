package proxy

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"strings"

	"github.com/INFURA/go-ethlibs/jsonrpc"
	"github.com/dominant-strategies/go-quai-stratum/util"
	"github.com/dominant-strategies/go-quai/common"
	"github.com/dominant-strategies/go-quai/consensus"
	"github.com/dominant-strategies/go-quai/core/types"
)

const (
	c_Max_Req_Size = 4096
)

func (s *ProxyServer) ListenTCP() {
	timeout := util.MustParseDuration(s.config.Proxy.Stratum.Timeout)
	s.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp4", s.config.Proxy.Stratum.Listen)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer server.Close()

	log.Printf("Stratum listening on %s", s.config.Proxy.Stratum.Listen)
	var accept = make(chan int, s.config.Proxy.Stratum.MaxConn)

	n := 0
	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) {
			conn.Close()
			continue
		}
		n += 1
		cs := &Session{conn: conn, ip: ip}

		accept <- n
		go func(cs *Session) {
			err = s.handleTCPClient(cs)
			if err != nil {
				s.removeSession(cs)
				conn.Close()
			}
			<-accept
		}(cs)
	}
}

func (s *ProxyServer) handleTCPClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connbuff := bufio.NewReaderSize(cs.conn, c_Max_Req_Size)
	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Printf("Socket flood detected from %s", cs.ip)
			cs.sendTCPError(jsonrpc.LimitExceeded(fmt.Sprintf("Message exceeds proxy's buffer size of %v", c_Max_Req_Size)))
			s.policy.BanClient(cs.ip)
			s.removeSession(cs)
			return err
		} else if err == io.EOF {
			log.Printf("Client %s disconnected", cs.ip)
			s.removeSession(cs)
			break
		} else if err != nil {
			log.Printf("Error reading from socket: %v", err)
			return err
		}

		if len(data) > 1 {
			var req jsonrpc.Request
			err = req.UnmarshalJSON(data)
			if err != nil {
				s.policy.ApplyMalformedPolicy(cs.ip)
				log.Print(data)
				log.Printf("Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}
			err = cs.handleTCPMessage(s, &req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cs *Session) handleTCPMessage(s *ProxyServer, req *jsonrpc.Request) error {
	// Handle RPC methods
	switch req.Method {
	case "mining.hello":
		log.Print("received hello")
		return nil
	case "quai_submitLogin":
		errReply := s.handleLoginRPC(cs, req.Params)
		if errReply != nil {
			return cs.sendTCPError(jsonrpc.MethodNotFound(req))
		}
		return nil
	case "quai_getPendingHeader":
		reply, errReply := s.handleGetWorkRPC(cs)
		if errReply != nil {
			return cs.sendTCPError(jsonrpc.NewError(-1, errReply.Message))
		}
		header_rep := reply.RPCMarshalHeader()
		cs.sendTCPResult(req.ID.String(), header_rep)
		return nil
	case "quai_receiveMinedHeader":
		var received_header *types.Header
		err := json.Unmarshal(req.Params[0], &received_header)
		if err != nil {
			log.Printf("Unable to decode header from %v. Err: %v", cs.ip, err)
			return err
		}
		s.submitMinedHeader(cs, received_header)

		return nil
	case "quai_getWork":
		log.Print("Getting work package for external miner")
		cs.pushNewJob(s.currentBlockTemplate().Header)
		return nil
	case "quai_submitWork":
		// Receives nonce, hash, and digest.
		var (
			received_nonce    types.BlockNonce
			received_mixHash  common.Hash
			received_sealHash common.Hash
		)

		log.Printf("Received work")

		err := json.Unmarshal(req.Params[0], &received_nonce)
		if err != nil {
			log.Printf("Unable to decode nonce from %v. Err: %v", cs.ip, err)
			return err
		}

		err = json.Unmarshal(req.Params[1], &received_mixHash)
		if err != nil {
			log.Printf("Unable to decode hash from %v. Err: %v", cs.ip, err)
			return err
		}

		err = json.Unmarshal(req.Params[2], &received_sealHash)
		if err != nil {
			log.Printf("Unable to decode digest from %v. Err: %v", cs.ip, err)
			return err
		}

		log.Printf("finished receive work")

		return nil
	case "quai_rawHeader":
		stripped := string(req.Params[0][1 : len(req.Params[0])-1])
		log.Print("Received new solution: ", stripped)

		received_nonce, _ := hex.DecodeString(stripped)
		blockNonce := types.BlockNonce(received_nonce)

		cur_header, errReply := s.handleGetWorkRPC(cs)
		if errReply != nil {
			log.Printf("Unable to get current header: %v", errReply)
			return nil
		}

		cur_header.SetNonce(blockNonce)

		hash := cur_header.Hash().Bytes()
		log.Printf("Hash: %x", hash)

		s.submitMinedHeader(cs, cur_header)

		return nil
	default:
		return cs.sendTCPError(jsonrpc.MethodNotFound(req))
	}
}

func (cs *Session) sendTCPResult(id string, result interface{}) error {
	cs.Lock()
	defer cs.Unlock()

	message := jsonrpc.Response{
		ID:     jsonrpc.StringID(id),
		Result: result,
		Error:  nil,
	}

	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJob(header *types.Header) error {
	cs.Lock()
	defer cs.Unlock()

	workPackage, err := consensus.MakeWork(header)
	if err != nil {
		log.Printf("Unable to make work: %v", err)
		return err
	}

	message := strings.Join(workPackage[:], "")

	bytes_written, err := cs.conn.Write([]byte(message))
	if err != nil {
		log.Fatalf("Unable to write to socket: %v", err)
		return err
	}
	log.Printf("Bytes written: %v", bytes_written)

	return nil
}

func (cs *Session) sendTCPError(err *jsonrpc.Error) error {
	cs.Lock()
	defer cs.Unlock()

	return cs.enc.Encode(err)
}

func (s *ProxyServer) registerSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[cs] = struct{}{}
}

func (s *ProxyServer) removeSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, cs)
}

func (s *ProxyServer) broadcastNewJobs() {
	t := s.currentBlockTemplate()
	if t == nil || t.Header == nil || t.Target == nil || s.isSick() {
		return
	}

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := len(s.sessions)
	log.Printf("Broadcasting new job to %v stratum miners", count)

	bcast := make(chan int, 1024)
	n := 0

	for m := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {

			err := cs.pushNewJob(t.Header)
			<-bcast
			if err != nil {
				log.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			}
		}(m)
	}
}

func (cs *Session) sendNewJob(header *types.Header, target *big.Int) {
	err := cs.pushNewJob(header)
	if err != nil {
		log.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
	}
}

func (s *ProxyServer) submitMinedHeader(cs *Session, header *types.Header) error {
	_, order, err := s.engine.CalcOrder(header)
	if err != nil {
		log.Print("Received header does not achieve minimum difficulty. Rejecting.")
		return err
	}

	// Should be synchronous starting with the lowest levels.
	log.Printf("Received a %s block", strings.ToLower(common.OrderToString(order)))

	// Send mined header to the relevant go-quai nodes.
	// Should be synchronous starting with the lowest levels.
	for i := common.HierarchyDepth - 1; i >= order; i-- {
		err := s.rpc(i).SubmitMinedHeader(header)
		if err != nil {
			// Header was rejected. Refresh workers to try again.
			log.Print("Rejected header.")
			cs.sendNewJob(header, s.currentBlockTemplate().Target)
			return err
		}
	}

	return nil
}
