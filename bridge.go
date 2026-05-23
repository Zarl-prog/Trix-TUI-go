package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

func sanitizePath(path string) string {
	return strings.ReplaceAll(path, "\x00", "")
}

type RPCRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
	ID     int         `json:"id,omitempty"`
}

type RPCResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

type RPCEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type Bridge struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
	id     int
	calls  map[int]chan *RPCResponse
	events chan RPCEvent
}

func NewBridge(pythonPath string) (*Bridge, error) {
	cmd := exec.Command(pythonPath, "core_bridge.py")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	b := &Bridge{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		calls:  make(map[int]chan *RPCResponse),
		events: make(chan RPCEvent, 100),
	}

	go b.readLoop()

	return b, nil
}

func (b *Bridge) readLoop() {
	scanner := bufio.NewScanner(b.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var resp RPCResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID != 0 {
			b.mu.Lock()
			if ch, ok := b.calls[resp.ID]; ok {
				ch <- &resp
				delete(b.calls, resp.ID)
			}
			b.mu.Unlock()
			continue
		}

		var event RPCEvent
		if err := json.Unmarshal(line, &event); err == nil && event.Event != "" {
			b.events <- event
			continue
		}
	}
}

func (b *Bridge) Call(method string, params interface{}) (json.RawMessage, error) {
	b.mu.Lock()
	b.id++
	id := b.id
	ch := make(chan *RPCResponse, 1)
	b.calls[id] = ch
	b.mu.Unlock()

	// Sanitize paths in params if it's a map
	if pMap, ok := params.(map[string]interface{}); ok {
		for k, v := range pMap {
			if s, ok := v.(string); ok && (strings.Contains(k, "path") || strings.Contains(k, "root")) {
				pMap[k] = sanitizePath(s)
			}
		}
	}

	req := RPCRequest{
		Method: method,
		Params: params,
		ID:     id,
	}

	data, _ := json.Marshal(req)
	fmt.Fprintln(b.stdin, string(data))

	resp := <-ch
	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}
	return resp.Result, nil
}

func (b *Bridge) Send(method string, params interface{}) error {
	// Sanitize paths in params if it's a map
	if pMap, ok := params.(map[string]interface{}); ok {
		for k, v := range pMap {
			if s, ok := v.(string); ok && (strings.Contains(k, "path") || strings.Contains(k, "root")) {
				pMap[k] = sanitizePath(s)
			}
		}
	}

	req := RPCRequest{
		Method: method,
		Params: params,
	}
	data, _ := json.Marshal(req)
	_, err := fmt.Fprintln(b.stdin, string(data))
	return err
}

func (b *Bridge) Events() <-chan RPCEvent {
	return b.events
}

func (b *Bridge) Close() {
	b.Send("quit", nil)
	b.cmd.Wait()
}
