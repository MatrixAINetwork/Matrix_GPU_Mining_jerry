package aidigger

/*
#cgo CFLAGS: -I./libdigger/src/
#cgo LDFLAGS: -L./libdigger/bin/static/ -lgold_digger -lm -pthread -lX11 -lssl -lcrypto -lstdc++ -ljpeg -lgomp
#include <stdlib.h>
#include "./libdigger/src/digger_interface.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"github.com/MatrixAINetwork/go-matrix/log"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"
)

var initDataPtr unsafe.Pointer

var (
	errNilTask     = errors.New("task is nil")
	errNoResultYet = errors.New("no ai digging result yet")
)

func Init(cfgPath string, pictures []string) {
	if initDataPtr == nil {
		charWeightsPath := C.CString(filepath.Join(cfgPath, "yolov3.weights"))
		defer C.free(unsafe.Pointer(charWeightsPath))
		charCfgPath := C.CString(filepath.Join(cfgPath, "yolov3.cfg"))
		defer C.free(unsafe.Pointer(charCfgPath))
		charNamesPath := C.CString(filepath.Join(cfgPath, "coco.names"))
		defer C.free(unsafe.Pointer(charNamesPath))

		cWeightsPath := (*C.char)(unsafe.Pointer(charWeightsPath))
		cCfgPath := (*C.char)(unsafe.Pointer(charCfgPath))
		cNamesPath := (*C.char)(unsafe.Pointer(charNamesPath))

		cPictures := make([]*C.char, 0)
		for i := range pictures {
			char := C.CString(pictures[i])
			defer C.free(unsafe.Pointer(char))
			strPtr := (*C.char)(unsafe.Pointer(char))
			cPictures = append(cPictures, strPtr)
		}

		log.Info("ai digger", "调用C库接口开始", "init_yolov3_data")
		initDataPtr = C.init_yolov3_data(cWeightsPath, cCfgPath, cNamesPath, (**C.char)(unsafe.Pointer(&cPictures[0])))
		log.Info("ai digger", "调用C库接口结束", "init_yolov3_data")
		if initDataPtr == nil {
			fatalf("init ai mining lib err!")
		}
	}
}

func AIDigging(seed int64, pictures []string, stopCh chan struct{}, resultCh chan []byte, errCh chan error) {
	cSeed := (C.long)(seed)
	cPictures := make([]*C.char, 0)
	for i := range pictures {
		char := C.CString(pictures[i])
		defer C.free(unsafe.Pointer(char))
		strPtr := (*C.char)(unsafe.Pointer(char))
		cPictures = append(cPictures, strPtr)
	}
	cThreadCount := (C.int)(0)
	log.Info("ai digger", "调用C库接口开始", "creat_thread")
	cThreadId, err := C.creat_thread(cSeed, (**C.char)(unsafe.Pointer(&cPictures[0])), initDataPtr, cThreadCount)
	//log.Info("ai digger", "调用C库接口结束", "creat_thread", "id", cThreadId, "seed", seed)
	if err != nil {
		log.Error("ai digger", "start ai digging err", err)
		errCh <- fmt.Errorf("start ai digging err: %v", err)
		return
	}
	beginTime := time.Now()
	log.Info("ai digger", "创建任务成功", cThreadId, "seed", seed, "time", beginTime)
	result := make([]byte, 32)
	for {
		select {
		case <-stopCh:
			log.Info("ai digger", "stop ai digging", cThreadId)
			log.Info("ai digger", "调用C库接口开始", "cancel_thread", "id", cThreadId)
			_, err := C.cancel_thread(cThreadId)
			if err != nil {
				log.Error("ai digger", "stop ai digging err", err)
				errCh <- fmt.Errorf("stop ai digging err: %v", err)
				return
			}
			return
		default:
			log.Info("ai digger", "调用C库接口开始", "get_result", "id", cThreadId)
			rst := C.get_result(cThreadId, (*C.uchar)(unsafe.Pointer(&result[0])))
			if rst == 1 {
				timePass := (time.Now().UnixNano() - beginTime.UnixNano()) / 1000000
				log.Info("ai digger", "get ai digging result", cThreadId, "result", result, "time pass(ms)", timePass)
				resultCh <- result
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func fatalf(format string, args ...interface{}) {
	w := io.MultiWriter(os.Stdout, os.Stderr)
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, _ := os.Stdout.Stat()
		errf, _ := os.Stderr.Stat()
		if outf != nil && errf != nil && os.SameFile(outf, errf) {
			w = os.Stderr
		}
	}
	fmt.Fprintf(w, "Fatal: "+format+"\n", args...)
	os.Exit(1)
}
