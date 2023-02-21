package proxy

import (
	"bufio"
	"encoding/json"
	// "errors"
	"io"
	"log"
	"net"
	"time"

	"github.com/INFURA/go-ethlibs/jsonrpc"
	"github.com/dominant-strategies/go-quai-stratum/rpc"
	"github.com/dominant-strategies/go-quai-stratum/util"
	"github.com/dominant-strategies/go-quai/core/types"
)

const (
	MaxReqSize = 4096
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
	log.Println("Received TCP dial")
	cs.enc = json.NewEncoder(cs.conn)
	connbuff := bufio.NewReaderSize(cs.conn, MaxReqSize)
	s.setDeadline(cs.conn)
	for {
		time.Sleep(1000)
		data, isPrefix, err := connbuff.ReadLine()
		log.Println("Received TCP data from client")
		if isPrefix {
			log.Printf("Socket flood detected from %s", cs.ip)
			s.policy.BanClient(cs.ip)
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
			// var req StratumReq
			var req jsonrpc.Request
			// err = jsonrpc.Unmarshal(data, &req)
			err = req.UnmarshalJSON(data)
			if err != nil {
				s.policy.ApplyMalformedPolicy(cs.ip)
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
	case "quai_submitLogin":
		// var params []json.RawMessage
		// var params jsonrpc.Params
		// err := json.Unmarshal(req.Params, &params)

		// if err != nil {
		// 	log.Printf("Malformed stratum request from %s: %v", cs.ip, err)
		// 	return err
		// }
		_, errReply := s.handleLoginRPC(cs, req.Params)
		if errReply != nil {
			return cs.sendTCPError(*jsonrpc.MethodNotFound(req))
		}
		// return cs.sendTCPResult(req.ID.String(), reply)
		// true_rep := jsonrpc.NewResponse()
		// true_rep.
		return nil
	case "quai_getPendingHeader":
		reply, errReply := s.handleGetWorkRPC(cs)
		if errReply != nil {
			// return cs.sendTCPError(req.ID.String(), errReply)
			return cs.sendTCPError(*jsonrpc.NewError(0, errReply.Message))
		}
		header_rep := rpc.RPCMarshalHeader(reply)
		log.Println(header_rep)
		cs.sendTCPResult(req.ID.String(), header_rep)
		return nil
	case "quai_receiveMinedHeader":
		// var params []string
		var received_header *types.Header
		// err := json.Unmarshal(req.Params, &received_header)
		err := json.Unmarshal(req.Params[0], &received_header)
		if err != nil {
			log.Println("Unable to decode header from ", cs.ip)
			// log.Println("Malformed stratum request params from", cs.ip)
			return err
		}
		return s.rpc().SubmitMinedHeader(received_header)
		// reply, errReply := s.handleTCPSubmitRPC(cs, req.Worker, received_header)
		// if errReply != nil {
			// return cs.sendTCPError(req.Id, errReply)
		// }
	case "eth_submitHashrate":
		return cs.sendTCPResult(req.ID.String(), true)
	default:
		// errReply := s.handleUnknownRPC(cs, req.Method)
		return cs.sendTCPError(*jsonrpc.MethodNotFound(req))
	}
}

func (cs *Session) sendHeaderTCP(id json.RawMessage, result *types.Header) ([]byte, error) {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	// return cs.enc.Encode(&message)
	return json.Marshal(message.Result)
}

func (cs *Session) sendTCPResult(id string, result interface{}) error {
	cs.Lock()
	defer cs.Unlock()

	// message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	message := jsonrpc.Response{
		ID:		jsonrpc.StringID(string(id)),
		Result:	result,
		Error:	nil,
	}

	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJob(result interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	message := JSONPushMessage{Version: "2.0", Result: result, Id: 0}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendTCPError(err jsonrpc.Error) error {
	cs.Lock()
	defer cs.Unlock()

	// message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	// err := cs.enc.Encode(&message)
	// if err != nil {
		// return err
	// }
	// return errors.New(reply.Message)
	return nil
}

func (*ProxyServer) setDeadline(conn *net.TCPConn) {
	// conn.SetDeadline(time.Now().Add(self.timeout))
	conn.SetDeadline(time.Time{})
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
	if t == nil || t.Header == nil || s.isSick() {
		return
	}
	// reply := []string{t.Header, t.Seed, s.diff}
	reply := t.Header

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := len(s.sessions)
	log.Printf("Broadcasting new job to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m, _ := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {
			err := cs.pushNewJob(&reply)
			<-bcast
			if err != nil {
				log.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			} else {
				s.setDeadline(cs.conn)
			}
		}(m)
	}
	log.Printf("Jobs broadcast finished %s", time.Since(start))
}
