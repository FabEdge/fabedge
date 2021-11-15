package memberlist

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"time"
)

type msgHandlerFun func(b []byte)
type notifyLeaveFun func(name string)

type broadcast struct {
	msg    []byte
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return true
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
}

type delegate struct{
	notifyMsg msgHandlerFun
	queue     *memberlist.TransmitLimitedQueue
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(b []byte) {
	if len(b) == 0 {
		return
	}
	d.notifyMsg(b)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	if d.queue.NumNodes == nil {
		return nil
	} else {
		return d.queue.GetBroadcasts(overhead, limit)
	}
}

func (d *delegate) LocalState(join bool) []byte {
	return []byte{}
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
}

type eventDelegate struct {
	notifyLeave notifyLeaveFun
}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	ed.notifyLeave(node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
}

type Client struct{
	list *memberlist.Memberlist
	delegate *delegate
}

func New(initMembers []string, msgHandler msgHandlerFun, leaveHandler notifyLeaveFun) (*Client, error) {
	conf := memberlist.DefaultLANConfig()

	 dg := &delegate{
		notifyMsg: msgHandler,
		queue:     &memberlist.TransmitLimitedQueue{RetransmitMult: 2},
	}
	conf.Delegate = dg

	conf.Events = &eventDelegate{
		notifyLeave: leaveHandler,
	}

	list, err := memberlist.Create(conf)
	if err != nil {
		return nil, err
	}

	if len(initMembers) < 1 {
		return nil, fmt.Errorf("at lease one known member is needed")
	}
	_, err = list.Join(initMembers)
	if err != nil {
		return nil, err
	}

	dg.queue.NumNodes = func() int {
		return list.NumMembers()
	}

	return &Client{
		list: list,
		delegate: dg,
	}, nil
}


func (c *Client) ListMembers() []*memberlist.Node {
	return c.list.Members()
}

func (c *Client) UpdateNode() error {
	return c.list.UpdateNode(time.Second * 3)
}

func (c *Client) Broadcast(b []byte) {
	c.delegate.queue.QueueBroadcast(&broadcast{
		msg:    b,
	})
}
