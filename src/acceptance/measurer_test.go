package main_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	uuid "github.com/satori/go.uuid"
)

type uptimeMeasurer struct {
	failedCount int
	totalCount  int

	cancelled chan struct{}

	key   string
	value string

	interval time.Duration

	client  *clientv3.Client
	stopped bool

	lock *sync.Mutex
}

func NewUptimeMeasurer(client *clientv3.Client, interval time.Duration) (*uptimeMeasurer, error) {
	guid := uuid.NewV4()
	key := fmt.Sprintf("test-key-%s", guid.String())
	value := fmt.Sprintf("test-value-%s", guid.String())

	ctx, cancel := context.WithTimeout(context.Background(), etcdOperationTimeout)
	defer cancel()
	_, err := client.Put(ctx, key, value)
	if err != nil {
		return nil, err
	}

	return &uptimeMeasurer{
		cancelled: make(chan struct{}),
		client:    client,
		interval:  interval,
		key:       key,
		value:     value,
		lock:      &sync.Mutex{},
	}, nil
}

func (u *uptimeMeasurer) Start() {
	go func() {
		timer := time.NewTimer(u.interval)
		for {
			timer.Reset(u.interval)

			select {
			case <-u.cancelled:
				return
			case <-timer.C:
				ctx, cancel := context.WithTimeout(context.Background(), etcdOperationTimeout)
				resp, err := u.client.Get(ctx, u.key)
				u.incrementTotalCount()
				cancel()
				if err != nil {
					u.incrementFailedCount()
					fmt.Printf("Encountered failure (#%d): %s\n", u.getFailedCount(), err.Error())
					continue
				}

				if len(resp.Kvs) != 1 {
					fmt.Printf("Encountered failure (#%d): Unexpected number of keys (expected 1 got %d)\n", u.getFailedCount(), len(resp.Kvs))
					u.incrementFailedCount()
					continue
				}

				for _, kv := range resp.Kvs {
					if string(kv.Key) != u.key || string(kv.Value) != u.value {
						u.incrementFailedCount()
						fmt.Printf("Encountered failure (#%d): Mismatching Values: expected %s got %s\n", u.getFailedCount(), u.value, string(kv.Value))
						break
					}
				}
			}
		}
	}()
}

func (u *uptimeMeasurer) Stop() {
	if u.isStopped() {
		return
	}

	u.cancelled <- struct{}{}
	close(u.cancelled)
	u.setStopped()
}

func (u *uptimeMeasurer) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), etcdOperationTimeout)
	defer cancel()
	_, err := u.client.Delete(ctx, u.key)
	return err
}

func (u *uptimeMeasurer) incrementTotalCount() {
	u.lock.Lock()
	defer u.lock.Unlock()

	u.totalCount++
}

func (u *uptimeMeasurer) incrementFailedCount() {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.failedCount++
}

func (u *uptimeMeasurer) getFailedCount() int {
	u.lock.Lock()
	defer u.lock.Unlock()

	return u.failedCount
}

func (u *uptimeMeasurer) Counts() (int, int) {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.totalCount, u.failedCount
}

func (u *uptimeMeasurer) ActualDeviation() float64 {
	u.lock.Lock()
	defer u.lock.Unlock()
	return float64(u.failedCount) / float64(u.totalCount)
}

func (u *uptimeMeasurer) setStopped() {
	u.lock.Lock()
	defer u.lock.Unlock()

	u.stopped = true
}

func (u *uptimeMeasurer) isStopped() bool {
	u.lock.Lock()
	defer u.lock.Unlock()

	return u.stopped
}
