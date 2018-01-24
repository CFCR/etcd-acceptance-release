package main_test

import (
	"context"
	"fmt"
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

	client *clientv3.Client
}

func NewUptimeMeasurer(client *clientv3.Client, interval time.Duration) (*uptimeMeasurer, error) {
	guid := uuid.NewV4()
	key := fmt.Sprintf("test-key-%s", guid.String())
	value := fmt.Sprintf("test-value-%s", guid.String())

	_, err := client.Put(context.Background(), key, value)
	if err != nil {
		return nil, err
	}

	return &uptimeMeasurer{
		cancelled: make(chan struct{}),
		client:    client,
		interval:  interval,
		key:       key,
		value:     value,
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
				u.totalCount++
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				resp, err := u.client.Get(ctx, u.key)
				cancel()
				if err != nil {
					u.failedCount++
					fmt.Printf("Encountered failure (#%d): %s\n", u.failedCount, err.Error())
					continue
				}

				if len(resp.Kvs) != 1 {
					fmt.Printf("Encountered failure (#%d): Too many keys (%d)", u.failedCount, len(resp.Kvs))
					u.failedCount++
					continue
				}

				for _, kv := range resp.Kvs {
					if string(kv.Key) != u.key || string(kv.Value) != u.value {
						u.failedCount++
						fmt.Printf("Encountered failure (#%d): Mismatching Values: expected %s got %s\n", u.failedCount, u.value, string(kv.Value))
						break
					}
				}
			}
		}
	}()
}

func (u *uptimeMeasurer) Stop() error {
	u.cancelled <- struct{}{}
	close(u.cancelled)

	_, err := u.client.Delete(context.Background(), u.key)
	return err
}

func (u uptimeMeasurer) Counts() (int, int) {
	return u.totalCount, u.failedCount
}

func (u uptimeMeasurer) ActualDeviation() float64 {
	return float64(u.failedCount) / float64(u.totalCount)
}
