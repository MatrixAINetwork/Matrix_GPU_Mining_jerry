package aidigger

import (
	"github.com/MatrixAINetwork/go-matrix/log"
	"strconv"
	"testing"
	"time"
)

func TestDigger(t *testing.T) {
	log.InitLog(5)
	pictures := make([]string, 0)
	for i := 0; i < 16; i++ {
		pictures = append(pictures, "D:\\pic\\test_"+strconv.Itoa(i)+".jpg")
	}
	stopCh := make(chan struct{})
	resultCh := make(chan []byte, 1)

	go digging(t, 54321, pictures, stopCh, resultCh)

	i := 0

loop:
	for {
		select {
		case result := <-resultCh:
			log.Info("挖矿结果获得", "result", result)
			return

		default:
			if i == 3 {
				log.Info("主动停止挖矿")
				close(stopCh)
				break loop
			}
			time.Sleep(time.Second)
			i++
		}
	}
	log.Info("结束")
}

func digging(t *testing.T, seed int64, pictures []string, stopCh chan struct{}, resultCh chan []byte) {
	log.Info("挖矿线程开始")
	defer log.Info("挖矿线程结束")

	err := AIDigging(seed, pictures, stopCh, resultCh)
	if err != nil {
		t.Fatalf("digging err: %v", err)
	}
}
