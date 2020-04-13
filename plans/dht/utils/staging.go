package utils

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

type Stager interface {
	Begin() error
	End() error
	Reset(name string)
}

type stager struct {
	ctx   context.Context
	seq   int
	total int
	name  string
	stage int
	ri    *RunInfo
	t     time.Time
}

func (s *stager) Reset(name string) {
	s.name = name
	s.stage = 0
}

func NewBatchStager(ctx context.Context, seq int, total int, name string, ri *RunInfo) *BatchStager {
	return &BatchStager{stager{
		ctx:   ctx,
		seq:   seq,
		total: total,
		name:  name,
		stage: 0,
		ri:    ri,
		t:     time.Now(),
	}}
}

type BatchStager struct{ stager }

func (s *BatchStager) Begin() error {
	s.stage += 1
	s.t = time.Now()
	return nil
}
func (s *BatchStager) End() error {
	// Signal that we're done
	stage := sync.State(s.name + strconv.Itoa(s.stage))

	t := time.Now()
	_, err := s.ri.Client.SignalEntry(s.ctx, stage)
	if err != nil {
		return err
	}

	s.ri.RunEnv.RecordMetric(&runtime.MetricDefinition{
		Name:           "signal " + string(stage),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	t = time.Now()

	err = <-s.ri.Client.MustBarrier(s.ctx, stage, s.total).C
	s.ri.RunEnv.RecordMetric(&runtime.MetricDefinition{
		Name:           "barrier" + string(stage),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	s.ri.RunEnv.RecordMetric(&runtime.MetricDefinition{
		Name:           "full " + string(stage),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(s.t).Nanoseconds()))
	return err
}
func (s *BatchStager) Reset(name string) { s.stager.Reset(name) }

func NewSinglePeerStager(ctx context.Context, seq int, total int, name string, ri *RunInfo) *SinglePeerStager {
	return &SinglePeerStager{BatchStager{stager{
		ctx:   ctx,
		seq:   seq,
		total: total,
		name:  name,
		stage: 0,
		ri:    ri,
		t:     time.Now(),
	}}}
}

type SinglePeerStager struct{ BatchStager }

func (s *SinglePeerStager) Begin() error {
	if err := s.BatchStager.Begin(); err != nil {
		return err
	}

	// Wait until it's out turn
	stage := sync.State(s.name + string(s.stage))
	return <-s.ri.Client.MustBarrier(s.ctx, stage, s.seq).C
}
func (s *SinglePeerStager) End() error {
	return s.BatchStager.End()
}
func (s *SinglePeerStager) Reset(name string) { s.stager.Reset(name) }

func NewGradualStager(ctx context.Context, seq int, total int, name string, ri *RunInfo, gradFn gradualFn) *GradualStager {
	return &GradualStager{BatchStager{stager{
		ctx:   ctx,
		seq:   seq,
		total: total,
		name:  name,
		stage: 0,
		ri:    ri,
		t:     time.Now(),
	}}, gradFn}
}

type gradualFn func(seq int) (int, int)

type GradualStager struct {
	BatchStager
	gradualFn
}

func (s *GradualStager) Begin() error {
	if err := s.BatchStager.Begin(); err != nil {
		return err
	}

	// Wait until it's out turn
	ourTurn, waitFor := s.gradualFn(s.seq)

	stageWait := sync.State(fmt.Sprintf("%s%d-%d", s.name, s.stage, ourTurn))
	stageNext := sync.State(fmt.Sprintf("%s%d-%d", s.name, s.stage, ourTurn+1))
	s.ri.RunEnv.RecordMessage("%d is waiting on %d from state %d", s.seq, waitFor, ourTurn)
	err := <-s.ri.Client.MustBarrier(s.ctx, stageWait, waitFor).C
	if err != nil {
		return err
	}
	s.ri.RunEnv.RecordMessage("%d is running", s.seq)
	_, err = s.ri.Client.SignalEntry(s.ctx, stageNext)

	return err
}

func (s *GradualStager) End() error {
	lastStage := sync.State(fmt.Sprintf("%s%d-end", s.name, s.stage))
	_, err := s.ri.Client.SignalEntry(s.ctx, lastStage)
	if err != nil {
		return err
	}
	total := s.ri.RunEnv.TestInstanceCount - 1
	s.ri.RunEnv.RecordMessage("%d is done - waiting for %d", s.seq, total)
	err = <-s.ri.Client.MustBarrier(s.ctx, lastStage, total).C
	s.ri.RunEnv.RecordMessage("%d passed the barrier", s.seq)
	return err
}

func (s *GradualStager) Reset(name string) { s.stager.Reset(name) }

func LinearGradualStaging(slope int) gradualFn {
	return func(seq int) (int, int) {
		slope := slope
		turnNum := int(math.Floor(float64(seq) / float64(slope)))
		waitFor := slope
		if turnNum == 0 {
			waitFor = 0
		}
		return turnNum, waitFor
	}
}

func ExponentialGradualStaging() gradualFn {
	return func(seq int) (int, int) {
		switch seq {
		case 0:
			return 0, 0
		case 1:
			return 1, 1
		default:
			turnNum := int(math.Floor(math.Log2(float64(seq)))) + 1
			waitFor := int(math.Exp2(float64(turnNum - 2)))
			return turnNum, waitFor
		}
	}
}

type NoStager struct{}

func (s *NoStager) Begin() error      { return nil }
func (s *NoStager) End() error        { return nil }
func (s *NoStager) Reset(name string) {}
