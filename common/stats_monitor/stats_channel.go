package stats_monitor

import "blobfuse2/common/log"

type StatsCollector struct {
	statsChannel chan interface{}
}

func NewStatsCollector() (*StatsCollector, error) {
	sc := &StatsCollector{}
	err := sc.init()
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (sc *StatsCollector) init() error {
	sc.statsChannel = make(chan interface{}, 100000)
	go sc.statsDumper()
	return nil
}

func (sc *StatsCollector) AddStats(stats interface{}) {
	sc.statsChannel <- stats
}

func (sc *StatsCollector) statsDumper() {
	i := 1
	for st := range sc.statsChannel {
		log.Debug("%v. Channel Stats: %v", i, st)
		i++
	}
}
