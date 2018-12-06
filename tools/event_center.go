package tools

import "github.com/DSiSc/craft/types"

type P2PTestEventCenter struct {
}

// subscriber subscribe specified eventType with eventFunc
func (*P2PTestEventCenter) Subscribe(eventType types.EventType, eventFunc types.EventFunc) types.Subscriber {
	return nil
}

// subscriber unsubscribe specified eventType
func (*P2PTestEventCenter) UnSubscribe(eventType types.EventType, subscriber types.Subscriber) (err error) {
	return nil
}

// notify subscriber of eventType
func (*P2PTestEventCenter) Notify(eventType types.EventType, value interface{}) (err error) {
	return nil
}

// notify specified eventFunc
func (*P2PTestEventCenter) NotifySubscriber(eventFunc types.EventFunc, value interface{}) {

}

// notify subscriber traversing all events
func (*P2PTestEventCenter) NotifyAll() (errs []error) {
	return nil
}

// unsubscrible all event
func (*P2PTestEventCenter) UnSubscribeAll() {
}
