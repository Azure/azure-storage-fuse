package internal

import (
	"blobfuse2/common/log"
	"fmt"
	"os"
	"sync"
	"syscall"
)

type ChannelReader func()

type StatsCollector struct {
	componentName string
	channel       chan ChannelMsg
	workerDone    sync.WaitGroup
	reader        ChannelReader
}

type Stats struct {
	ComponentName string
	Operation     string
	Value         map[string]string
}

type ChannelMsg struct {
	IsEvent   bool
	CompStats interface{}
}

var pipeFile = "/home/sourav/monitorPipe"
var mu sync.Mutex

func NewStatsCollector(componentName string, reader ChannelReader) (*StatsCollector, error) {
	sc := &StatsCollector{componentName: componentName}
	sc.channel = make(chan ChannelMsg, 100000)
	sc.reader = reader

	return sc, nil
}

func (sc *StatsCollector) Init() {
	sc.workerDone.Add(1)
	go sc.statsDumper()
}

func (sc *StatsCollector) Destroy() error {
	close(sc.channel)
	sc.workerDone.Wait()
	return nil
}

func (sc *StatsCollector) AddStats(stats ChannelMsg) {
	sc.channel <- stats
}

func (sc *StatsCollector) statsDumper() {
	defer sc.workerDone.Done()

	err := createPipe()
	if err != nil {
		log.Err("StatsManager::StatsDumper : [%v]", err)
		return
	}

	f, err := os.OpenFile(pipeFile, os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		log.Err("StatsManager::StatsDumper : unable to open pipe file [%v]", err)
		return
	}
	defer f.Close()

	for st := range sc.channel {
		log.Debug("StatsManager::StatsDumper : %v stats: %v", sc.componentName, st)
		if st.IsEvent {
			mu.Lock()
			_, err = f.WriteString(fmt.Sprintf("%v\n", st))
			if err != nil {
				log.Err("StatsManager::StatsDumper : Unable to write to pipe [%v]", err)
			}
			mu.Unlock()
		} else {
			// TODO : accumulate component level stats
		}
	}
}

func createPipe() error {
	_, err := os.Stat(pipeFile)
	if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipeFile, 0666)
		if err != nil {
			log.Err("StatsManager::createPipe : unable to create pipe [%v]", err)
			return err
		}
	} else if err != nil {
		log.Err("StatsManager::createPipe : [%v]", err)
		return err
	}
	return nil
}
