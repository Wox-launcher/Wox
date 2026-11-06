package plugin

import (
	"fmt"
	"wox/util"
)

const windowEventQueueCapacity = 128

// windowEventQueue supplies document and event identities to the shared inbox.
type windowEventQueue struct {
	queue *util.CoalescingQueue[WindowEvent]
	ready <-chan struct{}
}

// newWindowEventQueue keeps document and event type isolation outside the generic queue.
func newWindowEventQueue() *windowEventQueue {
	queue := util.NewCoalescingQueue(windowEventQueueCapacity, func(event WindowEvent) string {
		if event.CoalesceKey == "" {
			return ""
		}
		return fmt.Sprintf("%d:%s%d:%s%s", len(event.DocumentID), event.DocumentID, len(event.Type), event.Type, event.CoalesceKey)
	})
	return &windowEventQueue{queue: queue, ready: queue.Ready()}
}

func (q *windowEventQueue) push(event WindowEvent) bool { return q.queue.Push(event) }
func (q *windowEventQueue) pop() (WindowEvent, bool)    { return q.queue.Pop() }
func (q *windowEventQueue) close()                      { q.queue.Close() }
