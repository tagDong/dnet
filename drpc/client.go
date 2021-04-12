package drpc

import (
	"fmt"
	"github.com/yddeng/timer"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultRPCTimeout = 8 * time.Second

var ErrRPCTimeout = fmt.Errorf("drpc: rpc timeout. ")

// Call represents an active RPC.
type Call struct {
	reqNo    uint64
	callback func(interface{}, error)
	timer    timer.Timer
}

// Client represents an RPC Client.
// There may be multiple outstanding Calls associated
// with a single Client, and a Client may be used by
// multiple goroutines simultaneously.
type Client struct {
	reqNo    uint64         // serial number
	timerMgr timer.TimerMgr // timer
	pending  sync.Map       // map[uint64]*Call
}

// Call invokes the function synchronous, waits for it to complete, and returns its result and error status.
func (client *Client) Call(channel RPCChannel, method string, data interface{}, timeout time.Duration) (result interface{}, err error) {
	waitC := make(chan struct{})
	f := func(ret_ interface{}, err_ error) {
		result = ret_
		err = err_
		close(waitC)
	}
	if err := client.Go(channel, method, data, timeout, f); err != nil {
		return nil, err
	}
	<-waitC
	return
}

// Go invokes the function asynchronously.
func (client *Client) Go(channel RPCChannel, method string, data interface{}, timeout time.Duration, callback func(interface{}, error)) error {
	if callback == nil {
		return fmt.Errorf("drpc: Go callback == nil")
	}

	req := &Request{
		SeqNo:  atomic.AddUint64(&client.reqNo, 1),
		Method: method,
		Data:   data,
	}

	if err := channel.SendRequest(req); err != nil {
		return err
	}

	c := &Call{
		reqNo:    req.SeqNo,
		callback: callback,
	}

	c.timer = client.timerMgr.OnceTimer(timeout, func() {
		if _, ok := client.pending.Load(c.reqNo); ok {
			client.pending.Delete(c.reqNo)
			c.callback(nil, ErrRPCTimeout)
		}
		c.timer = nil
	})

	client.pending.Store(c.reqNo, c)
	return nil
}

// OnRPCResponse
func (client *Client) OnRPCResponse(resp *Response) error {
	v, ok := client.pending.Load(resp.SeqNo)
	if !ok {
		return fmt.Errorf("drpc: OnRPCResponse reqNo:%d is not found", resp.SeqNo)
	}

	call := v.(*Call)
	call.callback(resp.Data, nil)

	if call.timer != nil {
		call.timer.Stop()
	}
	client.pending.Delete(resp.SeqNo)
	return nil

}

// NewClient returns a new Client to handle requests to the
// set of services at the other end of the connection.
// It adds a timer manager to
func NewClient() *Client {
	return &Client{
		timerMgr: timer.NewTimeWheelMgr(time.Millisecond*50, 20),
	}
}

// NewClientWithTimerMgr is like NewClient but uses the specified timerMgr.
func NewClientWithTimerMgr(timerMgr timer.TimerMgr) *Client {
	return &Client{
		timerMgr: timerMgr,
	}
}
