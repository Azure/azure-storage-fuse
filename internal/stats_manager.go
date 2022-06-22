package internal

import "sync"

type ChannelReader func()

type StatsCollector struct {
	componentName string
	channel       chan interface{}
	workerDone    sync.WaitGroup
	reader        ChannelReader
}

func NewStatsCollector(componentName string, reader ChannelReader) (*StatsCollector, error) {
	sc := &StatsCollector{componentName: componentName}
	sc.channel = make(chan interface{}, 100000)
	sc.reader = reader

	return sc, nil
}

func (sc *StatsCollector) init() {
	sc.workerDone.Add(1)
	go sc.reader()
}

func (sc *StatsCollector) destroy() error {
	close(sc.channel)
	sc.workerDone.Wait()
	return nil
}

func (sc *StatsCollector) addStats(stats interface{}) {
	sc.channel <- stats
}
