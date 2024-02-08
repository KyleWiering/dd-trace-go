// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016 Datadog, Inc.

package tracer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/internal"
	globalinternal "gopkg.in/DataDog/dd-trace-go.v1/internal"
	"gopkg.in/DataDog/dd-trace-go.v1/internal/log"

	"github.com/stretchr/testify/assert"
)

type callType int64

const (
	callTypeGauge callType = iota
	callTypeIncr
	callTypeCount
	callTypeTiming
)

type testStatsdClient struct {
	mu          sync.RWMutex
	gaugeCalls  []testStatsdCall
	incrCalls   []testStatsdCall
	countCalls  []testStatsdCall
	timingCalls []testStatsdCall
	counts      map[string]int64
	tags        []string
	n           int
	closed      bool
	flushed     int
}

type testStatsdCall struct {
	name     string
	floatVal float64
	intVal   int64
	timeVal  time.Duration
	tags     []string
	rate     float64
}

func withStatsdClient(s globalinternal.StatsdClient) StartOption {
	return func(c *config) {
		c.statsdClient = s
	}
}

func (tg *testStatsdClient) addCount(name string, value int64) {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	if tg.counts == nil {
		tg.counts = make(map[string]int64)
	}
	tg.counts[name] += value
}

func (tg *testStatsdClient) Gauge(name string, value float64, tags []string, rate float64) error {
	return tg.addMetric(callTypeGauge, tags, testStatsdCall{
		name:     name,
		floatVal: value,
		tags:     make([]string, len(tags)),
		rate:     rate,
	})
}

func (tg *testStatsdClient) Incr(name string, tags []string, rate float64) error {
	tg.addCount(name, 1)
	return tg.addMetric(callTypeIncr, tags, testStatsdCall{
		name: name,
		tags: make([]string, len(tags)),
		rate: rate,
	})
}

func (tg *testStatsdClient) Count(name string, value int64, tags []string, rate float64) error {
	tg.addCount(name, value)
	return tg.addMetric(callTypeCount, tags, testStatsdCall{
		name:   name,
		intVal: value,
		tags:   make([]string, len(tags)),
		rate:   rate,
	})
}

func (tg *testStatsdClient) Timing(name string, value time.Duration, tags []string, rate float64) error {
	return tg.addMetric(callTypeTiming, tags, testStatsdCall{
		name:    name,
		timeVal: value,
		tags:    make([]string, len(tags)),
		rate:    rate,
	})
}

func (tg *testStatsdClient) addMetric(ct callType, tags []string, c testStatsdCall) error {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	copy(c.tags, tags)
	switch ct {
	case callTypeGauge:
		tg.gaugeCalls = append(tg.gaugeCalls, c)
	case callTypeIncr:
		tg.incrCalls = append(tg.incrCalls, c)
	case callTypeCount:
		tg.countCalls = append(tg.countCalls, c)
	case callTypeTiming:
		tg.timingCalls = append(tg.timingCalls, c)
	}
	tg.tags = tags
	tg.n++
	return nil
}

func (tg *testStatsdClient) Flush() error {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.flushed++
	return nil
}

func (tg *testStatsdClient) Close() error {
	tg.closed = true
	return nil
}

func (tg *testStatsdClient) GaugeCalls() []testStatsdCall {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	c := make([]testStatsdCall, len(tg.gaugeCalls))
	copy(c, tg.gaugeCalls)
	return c
}

func (tg *testStatsdClient) IncrCalls() []testStatsdCall {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	c := make([]testStatsdCall, len(tg.incrCalls))
	copy(c, tg.incrCalls)
	return c
}

func (tg *testStatsdClient) CountCalls() []testStatsdCall {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	c := make([]testStatsdCall, len(tg.countCalls))
	copy(c, tg.countCalls)
	return c
}

func (tg *testStatsdClient) CallNames() []string {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	var n []string
	for _, c := range tg.gaugeCalls {
		n = append(n, c.name)
	}
	for _, c := range tg.incrCalls {
		n = append(n, c.name)
	}
	for _, c := range tg.countCalls {
		n = append(n, c.name)
	}
	for _, c := range tg.timingCalls {
		n = append(n, c.name)
	}
	return n
}

func (tg *testStatsdClient) CallsByName() map[string]int {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	counts := make(map[string]int)
	for _, c := range tg.gaugeCalls {
		counts[c.name]++
	}
	for _, c := range tg.incrCalls {
		counts[c.name]++
	}
	for _, c := range tg.countCalls {
		counts[c.name]++
	}
	for _, c := range tg.timingCalls {
		counts[c.name]++
	}
	return counts
}

func (tg *testStatsdClient) Counts() map[string]int64 {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	c := make(map[string]int64)
	for key, value := range tg.counts {
		c[key] = value
	}
	return c
}

func (tg *testStatsdClient) Tags() []string {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	t := make([]string, len(tg.tags))
	copy(t, tg.tags)
	return t
}

func (tg *testStatsdClient) Reset() {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.gaugeCalls = tg.gaugeCalls[:0]
	tg.incrCalls = tg.incrCalls[:0]
	tg.countCalls = tg.countCalls[:0]
	tg.timingCalls = tg.timingCalls[:0]
	tg.counts = make(map[string]int64)
	tg.tags = tg.tags[:0]
	tg.n = 0
}

// Wait blocks until n metrics have been reported using the testStatsdClient or until duration d passes.
// If d passes, or a wait is already active, an error is returned.
func (tg *testStatsdClient) Wait(asserts *assert.Assertions, n int, d time.Duration) error {
	c := func() bool {
		tg.mu.RLock()
		defer tg.mu.RUnlock()

		return tg.n >= n
	}
	if !asserts.Eventually(c, d, 50*time.Millisecond) {
		return fmt.Errorf("timed out after waiting %s for gauge events", d)
	}

	return nil
}

func TestReportContribMetrics(t *testing.T) {
	t.Run("gauge", func(t *testing.T) {
		var tg testStatsdClient
		trc := newUnstartedTracer(withStatsdClient(&tg))
		defer trc.statsd.Close()
		trc.wg.Add(1)
		c := make(chan internal.Stat)
		go func() {
			defer trc.wg.Done()
			trc.reportContribMetrics(time.Millisecond, c)
		}()
		s := internal.Stat{
			Name: "gauge",
			Kind: "gauge",
			Value: float64(1.0),
			Tags: nil,
			Rate: 1,
		}
		c <- s
		assert := assert.New(t)
		close(trc.stop)
		calls := tg.CallNames()
		assert.Contains(calls, "gauge")
	})
	t.Run("incompatible gauge", func (t *testing.T) {
		var tg testStatsdClient
		tp := new(log.RecordLogger)
		trc := newUnstartedTracer(withStatsdClient(&tg), WithLogger(tp), WithDebugMode(true))
		defer trc.statsd.Close()
		trc.wg.Add(1)

		c := make(chan internal.Stat)
		go func() {
			defer trc.wg.Done()
			trc.reportContribMetrics(time.Millisecond, c)
		}()

		s := internal.Stat{
			Name: "NotGauge",
			Kind: "gauge",
			Value: 1, // not a float64
			Tags: nil,
			Rate: 1,
		}
		c <- s
		assert := assert.New(t)
		close(trc.stop)
		calls := tg.CallNames()
		assert.NotContains(calls, "NotGauge")
		assert.Contains(tp.Logs()[0], "Contrib library submitted gauge stat with incompatible value; looking for float64 value but got int. Dropping stat NotGauge.")
	})
	t.Run("count", func(t *testing.T) {
		var tg testStatsdClient
		trc := newUnstartedTracer(withStatsdClient(&tg))
		defer trc.statsd.Close()
		trc.wg.Add(1)
		c := make(chan internal.Stat)
		go func() {
			defer trc.wg.Done()
			trc.reportContribMetrics(time.Millisecond, c)
		}()
		s := internal.Stat{
			Name: "count",
			Kind: "count",
			Value: int64(1),
			Tags: nil,
			Rate: 1,
		}
		c <- s
		assert := assert.New(t)
		close(trc.stop)
		calls := tg.CallNames()
		assert.Contains(calls, "count")
	})
	t.Run("incompatible count", func(t *testing.T) {
		var tg testStatsdClient
		tp := new(log.RecordLogger)
		trc := newUnstartedTracer(withStatsdClient(&tg), WithLogger(tp), WithDebugMode(true))
		defer trc.statsd.Close()
		trc.wg.Add(1)

		c := make(chan internal.Stat)
		go func() {
			defer trc.wg.Done()
			trc.reportContribMetrics(time.Millisecond, c)
		}()

		s := internal.Stat{
			Name: "NotCount",
			Kind: "count",
			Value: 1, //not int64
			Tags: nil,
			Rate: 1,
		}
		c <- s
		assert := assert.New(t)
		close(trc.stop)
		calls := tg.CallNames()
		assert.NotContains(calls, "count")
		fmt.Println(tp.Logs()[0])
		assert.Contains(tp.Logs()[0], "Contrib library submitted count stat with incompatible value; looking for int64 value but got int. Dropping stat NotCount.")
	})
	t.Run("incompatible kind", func (t *testing.T) {
		var tg testStatsdClient
		tp := new(log.RecordLogger)
		trc := newUnstartedTracer(withStatsdClient(&tg), WithLogger(tp), WithDebugMode(true))
		defer trc.statsd.Close()
		trc.wg.Add(1)

		c := make(chan internal.Stat)
		go func() {
			defer trc.wg.Done()
			trc.reportContribMetrics(time.Millisecond, c)
		}()

		s := internal.Stat{
			Name: "incompatible",
			Kind: "incompatible",
			Value: 100,
			Tags: nil,
			Rate: 1,
		}
		c <- s
		assert := assert.New(t)
		close(trc.stop)
		calls := tg.CallNames()
		assert.NotContains(calls, "incompatible")
		assert.Contains(tp.Logs()[0], "Contrib stat submission failed: metric type incompatible not supported")
	})
}

func TestReportRuntimeMetrics(t *testing.T) {
	var tg testStatsdClient
	trc := newUnstartedTracer(withStatsdClient(&tg))
	defer trc.statsd.Close()

	trc.wg.Add(1)
	go func() {
		defer trc.wg.Done()
		trc.reportRuntimeMetrics(time.Millisecond)
	}()
	assert := assert.New(t)
	err := tg.Wait(assert, 35, 1*time.Second)
	close(trc.stop)
	assert.NoError(err)
	calls := tg.CallNames()
	assert.True(len(calls) > 30)
	assert.Contains(calls, "runtime.go.num_cpu")
	assert.Contains(calls, "runtime.go.mem_stats.alloc")
	assert.Contains(calls, "runtime.go.gc_stats.pause_quantiles.75p")
}

func TestReportHealthMetrics(t *testing.T) {
	assert := assert.New(t)
	var tg testStatsdClient

	defer func(old time.Duration) { statsInterval = old }(statsInterval)
	statsInterval = time.Nanosecond

	tracer, _, flush, stop := startTestTracer(t, withStatsdClient(&tg))
	defer stop()

	tracer.StartSpan("operation").Finish()
	flush(1)
	tg.Wait(assert, 3, 10*time.Second)

	counts := tg.Counts()
	assert.Equal(int64(1), counts["datadog.tracer.spans_started"])
	assert.Equal(int64(1), counts["datadog.tracer.spans_finished"])
	assert.Equal(int64(0), counts["datadog.tracer.traces_dropped"])
}

func TestTracerMetrics(t *testing.T) {
	assert := assert.New(t)
	var tg testStatsdClient
	tracer, _, flush, stop := startTestTracer(t, withStatsdClient(&tg))

	tracer.StartSpan("operation").Finish()
	flush(1)
	tg.Wait(assert, 5, 100*time.Millisecond)

	calls := tg.CallsByName()
	counts := tg.Counts()
	assert.Equal(1, calls["datadog.tracer.started"])
	assert.True(calls["datadog.tracer.flush_triggered"] >= 1)
	assert.Equal(1, calls["datadog.tracer.flush_duration"])
	assert.Equal(1, calls["datadog.tracer.flush_bytes"])
	assert.Equal(1, calls["datadog.tracer.flush_traces"])
	assert.Equal(int64(1), counts["datadog.tracer.flush_traces"])
	assert.False(tg.closed)

	tracer.StartSpan("operation").Finish()
	stop()
	calls = tg.CallsByName()
	assert.Equal(1, calls["datadog.tracer.stopped"])
	assert.True(tg.closed)
}
