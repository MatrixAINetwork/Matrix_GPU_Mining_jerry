// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package amhash

import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"

	"github.com/MatrixAINetwork/go-matrix/aidigger"
	"github.com/MatrixAINetwork/go-matrix/baseinterface"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/consensus"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/log"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	aiPictureMaxCount = 64000 // AI图库数量
	aiPictureSize     = 16    // AI选取图片数量
)

// Seal implements consensus.Engine, attempting to find a nonce that satisfies
// the block's difficulty requirements.
func (amhash *Amhash) Seal(chain consensus.ChainReader, header *types.Header, stop <-chan struct{}, isBroadcastNode bool) (*types.Header, error) {
	log.INFO("seal", "挖矿", "开始", "高度", header.Number.Uint64())
	defer log.INFO("seal", "挖矿", "结束", "高度", header.Number.Uint64())
	curHeader := types.CopyHeader(header)
	// start ai mining first
	aiHash, stopped, err := amhash.aiMineProcess(chain, curHeader, stop, isBroadcastNode)
	if err != nil {
		return nil, err
	}
	if stopped {
		return nil, nil
	}
	curHeader.AIHash = aiHash

	// Create a runner and the multiple search threads it directs
	abort := make(chan struct{})
	found := make(chan *types.Header)
	amhash.lock.Lock()
	threads := amhash.threads
	if amhash.rand == nil {
		seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			amhash.lock.Unlock()
			return nil, err
		}
		amhash.rand = rand.New(rand.NewSource(seed.Int64()))
	}
	amhash.lock.Unlock()

	threads = runtime.NumCPU()
	if isBroadcastNode {
		threads = 1
	}

	var pend sync.WaitGroup
	for i := 0; i < threads; i++ {
		pend.Add(1)
		go func(id int, nonce uint64) {
			defer pend.Done()
			amhash.mine(curHeader, id, nonce, abort, found, isBroadcastNode)

		}(i, uint64(amhash.rand.Int63()))
	}
	// Wait until sealing is terminated or a nonce is found
	var result *types.Header
	select {
	case <-stop:
		log.INFO("SEALER", "Sealer receive stop mine, curHeader", curHeader.HashNoSignsAndNonce().TerminalString())
		// Outside abort, stop all miner threads
		close(abort)
	case result = <-found:
		// One of the threads found a block, abort all others
		close(abort)
	case <-amhash.update:
		// Thread count was changed on user request, restart
		close(abort)
		pend.Wait()
		return amhash.Seal(chain, curHeader, stop, isBroadcastNode)
	}

	// Wait for all miners to terminate and return the block
	pend.Wait()
	return result, nil
}

// mine is the actual proof-of-work miner that searches for a nonce starting from
// seed that results in correct final block difficulty.
func (amhash *Amhash) mine(header *types.Header, id int, seed uint64, abort chan struct{}, found chan *types.Header, isBroadcastNode bool) {
	// Extract some data from the header
	var (
		curHeader = types.CopyHeader(header)
		//mineData  = generateMineData(curHeader)
		target = new(big.Int).Div(maxUint256, header.Difficulty)
		number = curHeader.Number.Uint64()
	)
	if isBroadcastNode {
		target = maxUint256
	}
	// Start generating random nonces until we abort or find a good one
	log.INFO("SEALER begin mine", "target", target, "isBroadcast", isBroadcastNode, "number", number, "diff", header.Difficulty.Uint64())
	defer log.INFO("SEALER stop mine", "number", number, "diff", header.Difficulty.Uint64())
	var (
		attempts = int64(0)
		nonce    = seed
	)
	logger := log.New("miner", id)
	logger.Trace("Started pow mine search for new nonces", "seed", seed)
search:
	for {
		select {
		case <-abort:
			// Mining terminated, update stats and abort
			logger.Trace("pow mine nonce search aborted", "attempts", nonce-seed)
			amhash.hashrate.Mark(attempts)
			return

		default:
			// We don't have to update hash rate on every nonce, so update after after 2^X nonces
			attempts++
			if (attempts % (1 << 15)) == 0 {
				amhash.hashrate.Mark(attempts)
				attempts = 0
			}
			// Compute the PoW value of this nonce
			/*result := x11PowHash(mineData, nonce) //YYYYYYYYYYYYYYY
			if new(big.Int).SetBytes(result).Cmp(target) <= 0 {
				// Correct nonce found, create a new header with it
				header = types.CopyHeader(header)
				header.Nonce = types.EncodeNonce(nonce)
				// Seal and return a block (if still needed)
				select {
				case found <- header:
					logger.Trace("Ethash nonce found and reported", "attempts", nonce-seed, "nonce", nonce)
				case <-abort:
					logger.Trace("Ethash nonce found but discarded", "attempts", nonce-seed, "nonce", nonce)
				}
				break search
			}
			*/
			//YYYYYYYYYYYYYYYYYYY
			result := x11PowHashByTest(combinationData(curHeader))
			shareBig, ret := new(big.Int).SetString(strings.TrimPrefix(Reverse(result), "0x"), 16)
			if ret {
				if shareBig.Cmp(target) <= 0 {
					//log.Info("YYYYYYYYYYYYYYYYYYYYY11111111111", "cmp shareBig and target,sharebig", shareBig, "target", target)
					// Correct nonce found, create a new header with it
					//header = types.CopyHeader(header)
					//header.Nonce = types.EncodeNonce(nonce)
					// Seal and return a block (if still needed)
					select {
					case found <- curHeader:
						logger.Trace("Ethash nonce found and reported", "attempts", nonce-seed, "nonce", nonce)
					case <-abort:
						logger.Trace("Ethash nonce found but discarded", "attempts", nonce-seed, "nonce", nonce)
					}
					break search
				}
				//log.Info("YYYYYYYYYYYYYYYYYYYYY22222222222", "cmp shareBig and target,sharebig", shareBig, "target", target)
			} else {
				//log.Info("YYYYYYYYYYYYYYYYYYYYY3333333333", "new(big.Int).SetString err,number ", curHeader.Number)
			}
			//YYYYYYYYYendYYYYYY
			nonce++
			curHeader.Nonce = types.EncodeNonce(nonce)
			bytnonce, _ := curHeader.Nonce.MarshalText()
			Reverse(bytnonce)
			curHeader.Nonce.UnmarshalText(bytnonce)
		}
	}
}

func generateMineData(header *types.Header) []byte {
	data := header.HashNoNonce().Bytes()
	data = append(data, header.AIHash.Bytes()...)
	data = append(data, header.Coinbase[0:8]...)
	data = append(data, []byte{0, 0, 0, 0}...)
	return data
}

func (amhash *Amhash) aiMineProcess(chain consensus.ChainReader, header *types.Header, stop <-chan struct{}, isBroadcastNode bool) (common.Hash, bool, error) {
	if isBroadcastNode {
		return common.Hash{}, false, nil
	}

	abortCh := make(chan struct{}, 1)
	foundCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go amhash.startAIMining(chain, header, abortCh, foundCh, errCh)

	for {
		select {
		case <-stop:
			log.INFO("SEALER", "Sealer receive stop mine msg", "ai mine stop", "current", header.HashNoSignsAndNonce())
			close(abortCh)
			return common.Hash{}, true, nil

		case <-amhash.update:
			close(abortCh)
			return amhash.aiMineProcess(chain, header, stop, isBroadcastNode)

		case err := <-errCh:
			log.Warn("SEALER", "ai mining err", err)
			return common.Hash{}, false, err

		case result := <-foundCh:
			aiHash := common.BytesToHash(result)
			log.INFO("SEALER", "aiMineProcess", "get ai digging result", "AIHash", aiHash)
			return aiHash, false, nil
		}
	}
}

func (amhash *Amhash) startAIMining(chain consensus.ChainReader, header *types.Header, abort chan struct{}, found chan []byte, errCh chan error) {
	// get seed
	vrf := baseinterface.NewVrf()
	_, vrfValue, _ := vrf.GetVrfInfoFromHeader(header.VrfValue)
	seed := big.NewInt(0).Add(common.BytesToHash(vrfValue).Big(), header.Coinbase.Big()).Int64()
	log.Info("YYYYYYYYYYYYYYYY", "startAIMining()", seed)
	// get pictureList
	indexList := getRandNums(seed, aiPictureMaxCount, aiPictureSize)
	pictureList := amhash.packPictureListByIndex(indexList)

	aidigger.AIDigging(seed, pictureList, abort, found, errCh)
}

func (amhash *Amhash) packPictureListByIndex(indexList []int) []string {
	pictureList := make([]string, 0)
	for i := 0; i < 16; i++ {
		pictureList = append(pictureList, filepath.Join(amhash.config.PictureStorePath, "test_"+strconv.Itoa(i)+".jpg"))
	}
	return pictureList
}
