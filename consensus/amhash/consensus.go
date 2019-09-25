// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package amhash

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	"bytes"
	"encoding/binary"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/common/math"
	"github.com/MatrixAINetwork/go-matrix/consensus"
	"github.com/MatrixAINetwork/go-matrix/consensus/misc"
	"github.com/MatrixAINetwork/go-matrix/core/state"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/params"
	"github.com/MatrixAINetwork/go-matrix/params/manversion"
	"gopkg.in/fatih/set.v0"
	"strings"
)

// amhash proof-of-work protocol constants.
var (
	FrontierBlockReward    *big.Int = big.NewInt(5e+18) // Block reward in wei for successfully mining a block
	ByzantiumBlockReward   *big.Int = big.NewInt(3e+18) // Block reward in wei for successfully mining a block upward from Byzantium
	maxUncles                       = 2                 // Maximum number of uncles allowed in a single block
	allowedFutureBlockTime          = 15 * time.Second  // Max time from current time allowed for blocks, before they're considered future blocks
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	errLargeBlockTime    = errors.New("timestamp too big")
	errZeroBlockTime     = errors.New("timestamp equals parent's")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
	errCoinbase          = errors.New("invalid coinbase")
	errInvalidAIMine     = errors.New("invalid AI Mine Result")
)

// Author implements consensus.Engine, returning the header's coinbase as the
// proof-of-work verified author of the block.
func (amhash *Amhash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// stock Matrix amhash engine.
func (amhash *Amhash) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	// If we're running a full engine faking, accept any input as valid
	if amhash.config.PowMode == ModeFullFake {
		return nil
	}
	// Short circuit if the header is known, or it's parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// Sanity checks passed, do a proper verification
	return amhash.verifyHeader(chain, header, parent, false, seal)
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// stock Matrix amhash engine.
func (amhash *Amhash) VerifySignatures(signature []common.Signature) (bool, error) {
	// If we're running a full engine faking, accept any input as valid
	return true, nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (amhash *Amhash) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	// If we're running a full engine faking, accept any input as valid
	if amhash.config.PowMode == ModeFullFake || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

	// Spawn as many workers as allowed threads
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

	// Create a task channel and spawn the verifiers
	var (
		inputs = make(chan int)
		done   = make(chan int, workers)
		errors = make([]error, len(headers))
		abort  = make(chan struct{})
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = amhash.verifyHeaderWorker(chain, headers, seals, index)
				done <- index
			}
		}()
	}

	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
					// Reached end of headers. Stop sending to workers.
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}

func (amhash *Amhash) verifyHeaderWorker(chain consensus.ChainReader, headers []*types.Header, seals []bool, index int) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if chain.GetHeader(headers[index].Hash(), headers[index].Number.Uint64()) != nil {
		return nil // known block
	}
	return amhash.verifyHeader(chain, headers[index], parent, false, seals[index])
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of the stock Matrix amhash engine.
func (amhash *Amhash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// If we're running a full engine faking, accept any input as valid
	if amhash.config.PowMode == ModeFullFake {
		return nil
	}
	// Verify that there are at most 2 uncles included in this block
	if len(block.Uncles()) > maxUncles {
		return errTooManyUncles
	}
	// Gather the set of past uncles and ancestors
	uncles, ancestors := set.New(), make(map[common.Hash]*types.Header)

	number, parent := block.NumberU64()-1, block.ParentHash()
	for i := 0; i < 7; i++ {
		ancestor := chain.GetBlock(parent, number)
		if ancestor == nil {
			break
		}
		ancestors[ancestor.Hash()] = ancestor.Header()
		for _, uncle := range ancestor.Uncles() {
			uncles.Add(uncle.Hash())
		}
		parent, number = ancestor.ParentHash(), number-1
	}
	ancestors[block.Hash()] = block.Header()
	uncles.Add(block.Hash())

	// Verify each of the uncles that it's recent, but not an ancestor
	for _, uncle := range block.Uncles() {
		// Make sure every uncle is rewarded only once
		hash := uncle.Hash()
		if uncles.Has(hash) {
			return errDuplicateUncle
		}
		uncles.Add(hash)

		// Make sure the uncle has a valid ancestry
		if ancestors[hash] != nil {
			return errUncleIsAncestor
		}
		if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == block.ParentHash() {
			return errDanglingUncle
		}
		if err := amhash.verifyHeader(chain, uncle, ancestors[uncle.ParentHash], true, true); err != nil {
			return err
		}
	}
	return nil
}

// verifyHeader checks whether a header conforms to the consensus rules of the
// stock Matrix amhash engine.
// See YP section 4.3.4. "Block Header Validity"
func (amhash *Amhash) verifyHeader(chain consensus.ChainReader, header, parent *types.Header, uncle bool, seal bool) error {
	// Ensure that the header's extra-data section is of a reasonable size
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	// Verify the header's timestamp
	if uncle {
		if header.Time.Cmp(math.MaxBig256) > 0 {
			return errLargeBlockTime
		}
	} else {
		if header.Time.Cmp(big.NewInt(time.Now().Add(allowedFutureBlockTime).Unix())) > 0 {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time.Cmp(parent.Time) <= 0 {
		return errZeroBlockTime
	}
	// super header don't verify difficulty
	if header.IsSuperHeader() == false {
		// Verify the block's difficulty based in it's timestamp and parent's difficulty
		expected, err := amhash.CalcDifficulty(chain, string(header.Version), header.Time.Uint64(), parent)
		if err != nil {
			return fmt.Errorf("calc difficulty err : %v", err)
		}
		if expected.Cmp(header.Difficulty) != 0 {
			return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
		}
	}

	// Verify that the gas limit is <= 2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.GasLimit > cap {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, cap)
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}

	// Verify that the gas limit remains within allowed bounds
	diff := int64(parent.GasLimit) - int64(header.GasLimit)
	if diff < 0 {
		diff *= -1
	}
	limit := parent.GasLimit / params.GasLimitBoundDivisor

	if uint64(diff) >= limit || header.GasLimit < params.MinGasLimit {
		return fmt.Errorf("invalid gas limit: have %d, want %d += %d", header.GasLimit, parent.GasLimit, limit)
	}
	// Verify that the block number is parent's +1
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}
	// Verify the engine specific seal securing the block
	if seal {
		if err := amhash.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	// If all checks passed, validate any special fields for hard forks
	if err := misc.VerifyDAOHeaderExtraData(chain.Config(), header); err != nil {
		return err
	}
	if err := misc.VerifyForkHashes(chain.Config(), header, uncle); err != nil {
		return err
	}
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func (amhash *Amhash) CalcDifficulty(chain consensus.ChainReader, curVersion string, time uint64, parent *types.Header) (*big.Int, error) {
	minDifficulty, err := chain.GetMinDifficulty(parent.Hash())
	if err != nil {
		return nil, err
	}
	return CalcDifficulty(chain.Config(), curVersion, time, parent, minDifficulty), nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func CalcDifficulty(config *params.ChainConfig, curVersion string, time uint64, parent *types.Header, minimumDifficulty *big.Int) *big.Int {
	logger := log.New("CalcDifficulty block", parent.Number)
	next := new(big.Int).Add(parent.Number, big1)
	switch {
	case config.IsByzantium(next):
		logger.Info("config.IsByzantium")
		return calcDifficultyByzantium(curVersion, time, parent, minimumDifficulty)
	case config.IsHomestead(next):
		logger.Info("config.IsHomestead")
		return calcDifficultyHomestead(time, parent, minimumDifficulty)
	default:
		logger.Info("config.IsHomestead")
		return calcDifficultyFrontier(time, parent, minimumDifficulty)
	}
}

// Some weird constants to avoid constant memory allocs for them.
var (
	expDiffPeriod = big.NewInt(100000)
	big1          = big.NewInt(1)
	big2          = big.NewInt(2)
	big9          = big.NewInt(9)
	big10         = big.NewInt(10)
	bigMinus99    = big.NewInt(-99)
	big2999999    = big.NewInt(2999999)
)

// calcDifficultyByzantium is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time given the
// parent block's time and difficulty. The calculation uses the Byzantium rules.
func calcDifficultyByzantium(curVersion string, time uint64, parent *types.Header, minimumDifficulty *big.Int) *big.Int {
	// https://github.com/MatrixAINetwork/EIPs/issues/100.
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
	//        ) + 2^(periodCount - 2)

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)

	// holds intermediate values to make the algo easier to read & audit
	x := new(big.Int)
	y := new(big.Int)
	logger := log.New("CalcDifficulty diff", parent.Difficulty)
	// (2 if len(parent_uncles) else 1) - (block_timestamp - parent_timestamp) // 9
	x.Sub(bigTime, bigParentTime)
	var durationLimit *big.Int
	if manversion.VersionCmp(curVersion, manversion.VersionGamma) >= 0 {
		durationLimit = params.VersionGammaDurationLimit
	} else {
		durationLimit = params.DurationLimit
	}
	logger.Info("CalcDifficulty diff", "duration", durationLimit.String())
	x.Div(x, durationLimit)
	if parent.UncleHash == types.EmptyUncleHash {
		x.Sub(big1, x)
	} else {
		x.Sub(big2, x)
	}
	// max((2 if len(parent_uncles) else 1) - (block_timestamp - parent_timestamp) // 9, -99)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	// parent_diff + (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	if y.Sign() == 0 {
		y = big1
	}
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)
	logger.Info("cal Diff", "x", x, "y", y, "minDiff", minimumDifficulty)
	// minimum difficulty can ever be (before exponential factor)
	if x.Cmp(minimumDifficulty) < 0 {
		x.Set(minimumDifficulty)
	}
	// calculate a fake block number for the ice-age delay:
	//   https://github.com/MatrixAINetwork/EIPs/pull/669
	//   fake_block_number = min(0, block.number - 3_000_000
	fakeBlockNumber := new(big.Int)
	if parent.Number.Cmp(big2999999) >= 0 {
		fakeBlockNumber = fakeBlockNumber.Sub(parent.Number, big2999999) // Note, parent is 1 less than the actual block number
	}
	// for the exponential factor
	periodCount := fakeBlockNumber
	periodCount.Div(periodCount, expDiffPeriod)

	// the exponential factor, commonly referred to as "the bomb"
	// diff = diff + 2^(periodCount - 2)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}

// calcDifficultyHomestead is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time given the
// parent block's time and difficulty. The calculation uses the Homestead rules.
func calcDifficultyHomestead(time uint64, parent *types.Header, minimumDifficulty *big.Int) *big.Int {
	// https://github.com/MatrixAINetwork/EIPs/blob/master/EIPS/eip-2.md
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	//        ) + 2^(periodCount - 2)

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)

	// holds intermediate values to make the algo easier to read & audit
	x := new(big.Int)
	y := new(big.Int)

	// 1 - (block_timestamp - parent_timestamp) // 10
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(big1, x)

	// max(1 - (block_timestamp - parent_timestamp) // 10, -99)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	// (parent_diff + parent_diff // 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)

	// minimum difficulty can ever be (before exponential factor)
	if x.Cmp(minimumDifficulty) < 0 {
		x.Set(minimumDifficulty)
	}
	// for the exponential factor
	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)

	// the exponential factor, commonly referred to as "the bomb"
	// diff = diff + 2^(periodCount - 2)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}

// calcDifficultyFrontier is the difficulty adjustment algorithm. It returns the
// difficulty that a new block should have when created at time given the parent
// block's time and difficulty. The calculation uses the Frontier rules.
func calcDifficultyFrontier(time uint64, parent *types.Header, minimumDifficulty *big.Int) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time)

	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(minimumDifficulty) < 0 {
		diff.Set(minimumDifficulty)
	}

	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		// diff = diff + 2^(periodCount - 2)
		expDiff := periodCount.Sub(periodCount, big2)
		expDiff.Exp(big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = math.BigMax(diff, minimumDifficulty)
	}
	return diff
}

// VerifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (amhash *Amhash) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	if err := amhash.verifyCoinbaseRole(chain, header); err != nil {
		return err
	}
	// If we're running a fake PoW, accept any seal as valid
	if amhash.config.PowMode == ModeFake || amhash.config.PowMode == ModeFullFake {
		time.Sleep(amhash.fakeDelay)
		if amhash.fakeFail == header.Number.Uint64() {
			return errInvalidPoW
		}
		return nil
	}
	// If we're running a shared PoW, delegate verification to it
	if amhash.shared != nil {
		return amhash.shared.VerifySeal(chain, header)
	}

	// verify ai mining first
	aiHash, _, err := amhash.aiMineProcess(chain, header, make(chan struct{}), false)
	if err != nil {
		return fmt.Errorf("ai mine process err: %v", err)
	}
	log.Info("YYYYYYYYYYYYYYYYY", "header hash", header.Hash(), "number", header.Number)
	if aiHash != header.AIHash {
		return errInvalidAIMine
	}

	// Ensure that we have a valid difficulty for the block
	if header.Difficulty.Sign() <= 0 {
		return errInvalidDifficulty
	}
	//-------------- 为了和矿机一致
	curHeader := types.CopyHeader(header)
	bytnonce, _ := header.Nonce.MarshalText()
	Reverse(bytnonce)
	header.Nonce.UnmarshalText(bytnonce)
	curHeader.Nonce = header.Nonce
	//---------------
	//result := x11PowHash(generateMineData(header), header.Nonce.Uint64()) //YYYYYYYYYYYY
	result := x11PowHashByTest(combinationData(curHeader))
	target := new(big.Int).Div(maxUint256, header.Difficulty)
	shareBig, ret := new(big.Int).SetString(strings.TrimPrefix(Reverse(result), "0x"), 16)
	if ret {
		if shareBig.Cmp(target) > 0 {
			log.Info("YYYYYYYYYYYYYYYYYY", "shareBig", shareBig, "target", target)
			return errInvalidPoW
		}
	} else {
		log.Info("YYYYYYYYYYYYYYYYYY", "result", result, "target", target, "err", ret)
		return errInvalidPoW
	}
	log.Info("YYYYYYYYYYYYYYYYY", "区块验证通过 header number", header.Number)
	return nil
}

//YYYYYYYYYYYYYYYYYYYYYYYYYYY
func combinationData(originalheader *types.Header) []byte {
	header := types.CopyHeader(originalheader)
	headerHash := hashString2LittleEndian(reverseHashString(header.HashNoNonce().Hex()))
	aiHash := hashString2LittleEndian(reverseHashString(header.AIHash.Hex()))
	nonce, _ := header.Nonce.MarshalText()
	Reverse(nonce)
	header.Nonce.UnmarshalText(nonce)
	nonce32 := uint32ToBytes(uint32(header.Nonce.Uint64()))
	databufio := bytes.NewBuffer([]byte{})
	binary.Write(databufio, binary.BigEndian, common.HexToHash(headerHash))
	binary.Write(databufio, binary.BigEndian, common.HexToHash(aiHash))
	binary.Write(databufio, binary.BigEndian, uint64(0)) //coinbase crc64
	binary.Write(databufio, binary.BigEndian, uint32(0)) //extranonce
	binary.Write(databufio, binary.LittleEndian, nonce32)
	return databufio.Bytes()
}

//YYYYYYYYYYYYYYYYYYYYYYYYYYY
//func combinationData(header *types.Header) []byte {
//	headerHash := hashString2LittleEndian(reverseHashString(header.ParentHash.Hex()))
//	aiHash := hashString2LittleEndian(reverseHashString(header.AIHash.Hex()))
//	//tmpnonce2 := header.Nonce
//	nonce, _ := header.Nonce.MarshalText()
//	Reverse(nonce)
//	header.Nonce.UnmarshalText(nonce)
//	nonce32 := uint32ToBytes(uint32(header.Nonce.Uint64()))
//	//log.Info("==testID==", "yuan shi nonce", tmpnonce2.Uint64(), "fan zhuan hou de header.Nonce", header.Nonce.Uint64(), "nonce32", nonce32, "headerHash", headerHash)
//	databufio := bytes.NewBuffer([]byte{})
//	binary.Write(databufio, binary.BigEndian, common.HexToHash(headerHash))
//	binary.Write(databufio, binary.BigEndian, common.HexToHash(aiHash))
//	binary.Write(databufio, binary.BigEndian, uint64(0)) //coinbase crc64
//	binary.Write(databufio, binary.BigEndian, uint32(0)) //extranonce
//	binary.Write(databufio, binary.LittleEndian, nonce32)
//	return databufio.Bytes()
//}
func Reverse(s []byte) string {
	//fmt.Println("len(s)", len(s), "s", s)
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return common.ToHex(s)
}
func reverseHashString(str string) string {
	str = strings.TrimPrefix(str, "0x")
	source := []byte(str)
	len := strings.Count(str, "") - 1
	i := len
	dest := make([]byte, len, len)
	for j := 0; j < len; {
		copy(dest[j:j+8], source[i-8:i])
		i = i - 8
		j = j + 8
	}
	return string(dest[:])
}
func hashString2LittleEndian(hash string) string {
	hash = strings.TrimPrefix(hash, "0x")
	in := common.FromHex(hash)
	ret := make([]byte, 32, 32)
	for k := 0; k < 32; {
		r := in[k : k+4]
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		copy(ret[k:k+4], r[:])
		k = k + 4
	}

	return strings.TrimPrefix(common.ToHex(ret[:]), "0x")
}

//YYYYYYYYYYY end YYYYYYYYYYY
// Prepare implements consensus.Engine, initializing the difficulty field of a
// header to conform to the amhash protocol. The changes are done inline.
func (amhash *Amhash) Prepare(chain consensus.ChainReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	difficulty, err := amhash.CalcDifficulty(chain, string(header.Version), header.Time.Uint64(), parent)
	if err != nil {
		return fmt.Errorf("calc difficulty err : %v", err)
	}
	header.Difficulty = difficulty
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (amhash *Amhash) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDBManage, uncles []*types.Header, currencyBlock []types.CurrencyBlock) (*types.Block, error) {
	// Accumulate any block and uncle rewards and commit the final state root
	//	accumulateRewards(chain.Config(), state, header, uncles)
	header.Roots, header.Sharding = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

	// Header seems complete, assemble into a block and return
	return types.NewBlock(header, currencyBlock, uncles), nil
}

// Some weird constants to avoid constant memory allocs for them.
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func accumulateRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header) {
	// Select the correct block reward based on chain progression
	blockReward := FrontierBlockReward
	if config.IsByzantium(header.Number) {
		blockReward = ByzantiumBlockReward
	}
	// Accumulate the rewards for the miner and any included uncles
	reward := new(big.Int).Set(blockReward)
	r := new(big.Int)
	for _, uncle := range uncles {
		r.Add(uncle.Number, big8)
		r.Sub(r, header.Number)
		r.Mul(r, blockReward)
		r.Div(r, big8)
		state.AddBalance(common.MainAccount, uncle.Coinbase, r)

		r.Div(blockReward, big32)
		reward.Add(reward, r)
	}
	state.AddBalance(common.MainAccount, header.Coinbase, reward)
}

func (amhash *Amhash) verifyCoinbaseRole(chain consensus.ChainReader, header *types.Header) error {
	//log.DEBUG("seal coinbase", "开始验证coinbase", header.Coinbase.Hex(), "高度", header.Number, "hash", header.Hash().Hex())
	preTopology, _, err := chain.GetGraphByHash(header.ParentHash)
	if err != nil {
		log.Error("seal coinbase", "get pre topology graph err", err)
		return errCoinbase
	}
	if preTopology.CheckAccountRole(header.Coinbase, common.RoleMiner) {
		return nil
	}

	innerMiners, err := chain.GetInnerMinerAccounts(header.ParentHash)
	if err != nil {
		log.Error("seal coinbase", "get inner miner accounts err", err)
		return errCoinbase
	}
	for _, account := range innerMiners {
		if account == header.Coinbase {
			return nil
		}
	}
	return errCoinbase
}
