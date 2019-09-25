// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package miner

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MatrixAINetwork/go-matrix/base58"
	"github.com/MatrixAINetwork/go-matrix/baseinterface"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/consensus"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/log"
	"math/big"
	"strings"
)

type hashrate struct {
	ping time.Time
	rate uint64
}

type RemoteAgent struct {
	mu sync.Mutex

	quitCh   chan struct{}
	workCh   chan *Work
	returnCh chan<- *types.Header

	chain       consensus.ChainReader
	engine      map[string]consensus.Engine
	currentWork *Work
	work        map[common.Hash]*Work

	hashrateMu sync.RWMutex
	hashrate   map[common.Hash]hashrate

	running int32 // running indicates whether the agent is active. Call atomically
}

func NewRemoteAgent(chain consensus.ChainReader, engine map[string]consensus.Engine) *RemoteAgent {
	return &RemoteAgent{
		chain:    chain,
		engine:   engine,
		work:     make(map[common.Hash]*Work),
		hashrate: make(map[common.Hash]hashrate),
	}
}

func (a *RemoteAgent) SubmitHashrate(id common.Hash, rate uint64) {
	a.hashrateMu.Lock()
	defer a.hashrateMu.Unlock()

	a.hashrate[id] = hashrate{time.Now(), rate}
}

func (a *RemoteAgent) Work() chan<- *Work {
	return a.workCh
}

func (a *RemoteAgent) SetReturnCh(returnCh chan<- *types.Header) {
	a.returnCh = returnCh
}

func (a *RemoteAgent) Start() {
	if !atomic.CompareAndSwapInt32(&a.running, 0, 1) {
		return
	}
	a.quitCh = make(chan struct{})
	a.workCh = make(chan *Work, 1)
	go a.loop(a.workCh, a.quitCh)
}

func (a *RemoteAgent) Stop() {
	if !atomic.CompareAndSwapInt32(&a.running, 1, 0) {
		return
	}
	close(a.quitCh)
	close(a.workCh)
}

// GetHashRate returns the accumulated hashrate of all identifier combined
func (a *RemoteAgent) GetHashRate() (tot int64) {
	a.hashrateMu.RLock()
	defer a.hashrateMu.RUnlock()

	// this could overflow
	for _, hashrate := range a.hashrate {
		tot += int64(hashrate.rate)
	}
	return
}

func (a *RemoteAgent) GetWork() ([3]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var res [3]string

	if a.currentWork != nil {
		block := a.currentWork.header

		res[0] = block.HashNoNonce().Hex()
		vrf := baseinterface.NewVrf()
		_, vrfValue, _ := vrf.GetVrfInfoFromHeader(block.VrfValue)
		seed := common.ToHex(vrfValue)
		//log.Info("YYYYYYYYYYYYYYYY", "getwork()", seed)
		res[1] = seed
		n := block.Difficulty
		res[2] = common.BytesToHash(n.Bytes()).Hex()

		a.work[block.HashNoNonce()] = a.currentWork
		return res, nil
	}
	return res, errors.New("No work available yet, don't panic.")
}

// SubmitWork tries to inject a pow solution into the remote agent, returning
// whether the solution was accepted or not (not can be both a bad pow as well as
// any other error, like no work pending).
func (a *RemoteAgent) SubmitWork(strnonce, strAIHah, strhash, strminerAddr string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	log.Info("YYYYYYYYYYYYYYYYYYYY", "nonce", strnonce, "AIHah", strAIHah, "header hash", strhash, "addr", strminerAddr)
	if len(strhash) != 66 || len(strAIHah) != 66 {
		log.Error("SubmitWork", "SubmitWork err, hash length wrong. nonce", strnonce, "AIHah", strAIHah, "header hash", strhash)
		return false
	}
	if len(strnonce) > 10 {
		log.Error("SubmitWork", "SubmitWork err, nonce too long ", strnonce)
		return false
	}
	hash := common.HexToHash(strhash)
	AIHah := common.HexToHash(strAIHah)
	nonce, err := conversType(strnonce)
	if err != nil {
		log.Info("YYYYYYYYYYYYYYYYYYYY", "submitWork,recv nonce len less 8")
		return false
	}
	// Make sure the work submitted is present
	work := a.work[hash]
	if work == nil {
		log.Info("Work submitted but none pending", "hash", hash)
		return false
	}
	addr, err := base58.Base58DecodeToAddress(strminerAddr)
	if err != nil {
		log.Info("Work submitted but miner address", "err", err, "miner address", strminerAddr)
		return false
	}
	// Make sure the Engine solutions is indeed valid
	result := types.CopyHeader(work.header)
	result.Nonce = nonce
	result.AIHash = AIHah
	result.Coinbase = addr

	engine, exist := a.engine[string(result.Version)]
	if exist == false {
		log.Warn("SubmitWork err", "header version can't find engine", string(result.Version))
		return false
	}

	log.Info("test log", "verify header", result.Hash(), "number", result.Number)
	if err := engine.VerifySeal(a.chain, result); err != nil {
		log.Warn("Invalid proof-of-work submitted", "hash", hash, "err", err)
		return false
	}
	//block := work.Block.WithSeal(result)

	// Solutions seems to be valid, return to the miner and notify acceptance
	a.returnCh <- result

	delete(a.work, hash)

	return true
}

//func conversType(str string) (types.BlockNonce, error) {
//	nonce := types.BlockNonce{}
//	//strnonce := strings.TrimPrefix(str, "0x")
//	strnonce := common.FromHex(str)
//	//if len(strnonce) == 8 {
//	for i := 0; i < len(strnonce); i++ {
//		nonce[i] = strnonce[i] //- '0'
//	}
//	//} else {
//	//	return types.BlockNonce{}, errors.New("submitWork,recv nonce len less 8")
//	//}
//	return nonce, nil
//}
func conversType(str string) (types.BlockNonce, error) {
	noncebig, _ := new(big.Int).SetString(strings.TrimPrefix(str, "0x"), 16)
	return types.EncodeNonce(noncebig.Uint64()), nil
}

// loop monitors mining events on the work and quit channels, updating the internal
// state of the remote miner until a termination is requested.
//
// Note, the reason the work and quit channels are passed as parameters is because
// RemoteAgent.Start() constantly recreates these channels, so the loop code cannot
// assume data stability in these member fields.
func (a *RemoteAgent) loop(workCh chan *Work, quitCh chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-quitCh:
			return
		case work := <-workCh:
			a.mu.Lock()
			a.currentWork = work
			a.mu.Unlock()
		case <-ticker.C:
			// cleanup
			/*
				a.mu.Lock()
				for hash, work := range a.work {
					if time.Since(work.createdAt) > 7*(12*time.Second) {
						delete(a.work, hash)
					}
				}
				a.mu.Unlock()
			*/ //YYYYYYYYYYYYYYYYYYYYYYYY 测试注释
			a.hashrateMu.Lock()
			for id, hashrate := range a.hashrate {
				if time.Since(hashrate.ping) > 10*time.Second {
					delete(a.hashrate, id)
				}
			}
			a.hashrateMu.Unlock()
		}
	}
}
