// This is for the client. JSON encodes args to POST to a peer

package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tristan/distributeddatabase/internal/raft"
)

// HTTPTransport implements raft.Transport using HTTP/JSON RPCs.
type HTTPTransport struct {
	peerAddrs map[string]string // peerID -> "host:port"
	client    *http.Client
}

func NewHTTPTransport(peerAddrs map[string]string) *HTTPTransport {
	return &HTTPTransport{
		peerAddrs: peerAddrs,
		client: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}
}

func (t *HTTPTransport) SendRequestVote(peerID string, args raft.RequestVoteArgs) (raft.RequestVoteReply, error) {
	var reply raft.RequestVoteReply
	err := t.rpc(peerID, "/raft/request-vote", args, &reply)
	return reply, err
}

func (t *HTTPTransport) SendAppendEntries(peerID string, args raft.AppendEntriesArgs) (raft.AppendEntriesReply, error) {
	var reply raft.AppendEntriesReply
	err := t.rpc(peerID, "/raft/append-entries", args, &reply)
	return reply, err
}

func (t *HTTPTransport) rpc(peerID, path string, args any, reply any) error {
	addr, ok := t.peerAddrs[peerID]
	if !ok {
		return fmt.Errorf("unknown peer: %s", peerID)
	}

	body, err := json.Marshal(args)
	if err != nil {
		return err
	}

	resp, err := t.client.Post(fmt.Sprintf("http://%s%s", addr, path), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rpc to %s%s returned %d", addr, path, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(reply)
}
