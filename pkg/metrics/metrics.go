package metrics

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Metrics struct {
	sync.RWMutex
	name        string
	time        int64
	rate        uint64
	rateRead    bool
	counter     uint64
	counterRead bool
	isDone      bool
}

func NewMetrics(name string) *Metrics {
	log.WithFields(log.Fields{"name": name}).Trace("New metrics created")
	return &Metrics{name: name, time: time.Now().Unix(), rate: 0, counter: 0, isDone: false, rateRead: false, counterRead: false}
}

func (m *Metrics) Countup() {
	m.Lock()
	defer m.Unlock()
	now := time.Now().Unix()
	if m.time != now {
		m.rate = m.counter
		m.counter = 0
		m.time = now
		m.rateRead = false
		log.WithFields(log.Fields{"name": m.name, "rate": m.rate}).Trace("[Metrics] updated rate")
	}
	m.counter++
	log.WithFields(log.Fields{"name": m.name, "counter": m.counter}).Trace("[Metrics] countuped")
}

func (m *Metrics) GetIsDone() bool {
	m.RLock()
	defer m.RUnlock()
	return m.isDone
}

func (m *Metrics) SetIsDone() {
	log.WithFields(log.Fields{"name": m.name}).Trace("[Metrics] called SetIsDone()")
	m.Lock()
	defer m.Unlock()
	m.isDone = true
}

func (m *Metrics) GetRate() (bool, uint64, string) {
	log.WithFields(log.Fields{"name": m.name}).Trace("[Metrics] called GetRate()")
	m.Lock()
	defer m.Unlock()
	if !m.isDone && m.rateRead {
		log.WithFields(log.Fields{
			"name":        m.name,
			"isDone":      m.isDone,
			"rateRead":    m.rateRead,
			"counterRead": m.counterRead,
		}).Debug("[Metrics] `rate` is not updated")
		return false, m.rate, m.name
	}
	if m.isDone && m.rateRead && !m.counterRead {
		log.WithFields(log.Fields{
			"name":        m.name,
			"isDone":      m.isDone,
			"rateRead":    m.rateRead,
			"counterRead": m.counterRead,
		}).Debug("[Metrics] will return `counter` not `rate`")
		m.counterRead = true
		return true, m.counter, m.name
	}
	if m.isDone && m.rateRead && m.counterRead {
		log.WithFields(log.Fields{
			"name":        m.name,
			"isDone":      m.isDone,
			"rateRead":    m.rateRead,
			"counterRead": m.counterRead,
		}).Debug("[Metrics] `rate` and `counter` are not updated")
		return false, 0, m.name
	}
	m.rateRead = true
	return true, m.rate, m.name
}
