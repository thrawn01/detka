package connection

import (
	"sync"
	"time"
)

type Manager struct {
	done      chan struct{}
	reconnect chan bool
	mutex     sync.Mutex
	value     interface{}
}

func (self *Manager) WithLock(callBack func()) {
	self.mutex.Lock()
	callBack()
	self.mutex.Unlock()
}

func (self *Manager) Signal() {
	self.reconnect <- true
}

func (self *Manager) Begin(callBack func() bool, waitTime time.Duration) {
	//fmt.Printf("Begin()\n")
	self.reconnect = make(chan bool)
	self.done = make(chan struct{})

	var attemptedConnect sync.WaitGroup

	go func() {
		var once sync.Once
		attemptedConnect.Add(1)

		for {
			var timer <-chan time.Time

			select {
			case <-self.reconnect:
				goto connect
			case <-timer:
				goto connect
			case <-self.done:
				return
			}
		connect:
			// Attempt to connect, if we fail to connect, set a timer to try again
			if !callBack() {
				timer = time.NewTimer(waitTime).C
			}
			once.Do(func() { attemptedConnect.Done() })
		}
	}()

	// Send the first connect signal
	self.Signal()
	// Attempt to connect at least once before we leave
	attemptedConnect.Wait()
}

func (self *Manager) End() {
	//fmt.Printf("Close()\n")
	close(self.done)
}
