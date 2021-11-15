package memberlist

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"time"
)

var (
	notifyMsg msgHandlerFun
	notifyLeave notifyLeaveFun
	broadcasts *memberlist.TransmitLimitedQueue
)

type msgHandlerFun func(b []byte)
type notifyLeaveFun func(name string)

type broadcast struct {
	msg    []byte
}

type eventDelegate struct{}
type delegate struct{}

type Client struct{
	list *memberlist.Memberlist
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return true
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(b []byte) {
	if len(b) == 0 {
		return
	}
	notifyMsg(b)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return broadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	return []byte{}
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	notifyLeave(node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
}

func New(msgHandler msgHandlerFun, leaveHandler notifyLeaveFun) (*Client, error) {
	notifyLeave = leaveHandler
	notifyMsg = msgHandler

	conf := memberlist.DefaultLANConfig()
	conf.Delegate = &delegate{}
	conf.Events = &eventDelegate{}

	list, err := memberlist.Create(conf)
	if err != nil {
		return nil, err
	}

	return &Client{
		list: list,
	}, nil
}

func (c *Client) ListMembers() []*memberlist.Node {
	return c.list.Members()
}

func (c *Client) UpdateNode() error {
	return c.list.UpdateNode(time.Second * 3)
}

func (c *Client) Run(initMembers []string) error {
	if len(initMembers) < 1 {
		return fmt.Errorf("at lease one known member is needed")
	}
	_, err := c.list.Join(initMembers)
	if err != nil {
		return err
	}

	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return c.list.NumMembers()
		},
		RetransmitMult: 3,
	}

	return nil
}

func (c *Client) Broadcast(b []byte) {
	broadcasts.QueueBroadcast(&broadcast{
		msg:    b,
	})
}
