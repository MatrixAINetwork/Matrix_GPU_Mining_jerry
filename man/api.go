// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package man

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MatrixAINetwork/go-matrix/reward/util"

	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/common/hexutil"
	"github.com/MatrixAINetwork/go-matrix/core"
	"github.com/MatrixAINetwork/go-matrix/core/rawdb"
	"github.com/MatrixAINetwork/go-matrix/core/state"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/mc"

	"github.com/MatrixAINetwork/go-matrix/base58"
	"github.com/MatrixAINetwork/go-matrix/man/wizard"
	"github.com/MatrixAINetwork/go-matrix/miner"
	"github.com/MatrixAINetwork/go-matrix/params"
	"github.com/MatrixAINetwork/go-matrix/rlp"
	"github.com/MatrixAINetwork/go-matrix/rpc"
	"github.com/MatrixAINetwork/go-matrix/trie"
)

// PublicMatrixAPI provides an API to access Matrix full node-related
// information.
type PublicMatrixAPI struct {
	e *Matrix
}

// NewPublicMatrixAPI creates a new Matrix protocol API for full nodes.
func NewPublicMatrixAPI(e *Matrix) *PublicMatrixAPI {
	return &PublicMatrixAPI{e}
}

// Manerbase is the address that mining rewards will be send to
func (api *PublicMatrixAPI) Manerbase() (common.Address, error) {
	return api.e.Manerbase()
}

// Coinbase is the address that mining rewards will be send to (alias for Manerbase)
func (api *PublicMatrixAPI) Coinbase() (common.Address, error) {
	return api.Manerbase()
}

// Hashrate returns the POW hashrate
func (api *PublicMatrixAPI) Hashrate() hexutil.Uint64 {
	return hexutil.Uint64(api.e.Miner().HashRate())
}

// PublicMinerAPI provides an API to control the miner.
// It offers only methods that operate on data that pose no security risk when it is publicly accessible.
type PublicMinerAPI struct {
	e     *Matrix
	agent *miner.RemoteAgent
}

// NewPublicMinerAPI create a new PublicMinerAPI instance.
func NewPublicMinerAPI(e *Matrix) *PublicMinerAPI {
	agent := miner.NewRemoteAgent(e.BlockChain(), e.EngineAll())
	e.Miner().Register(agent)

	return &PublicMinerAPI{e, agent}
}

// Mining returns an indication if this node is currently mining.
func (api *PublicMinerAPI) Mining() bool {
	return api.e.IsMining()
}

// SubmitWork can be used by external miner to submit their POW solution. It returns an indication if the work was
// accepted. Note, this is not an indication if the provided work was valid!
func (api *PublicMinerAPI) SubmitWork(nonce, solution, digest, minerAddr string) bool {
	return api.agent.SubmitWork(nonce, solution, digest, minerAddr)
}

// GetWork returns a work package for external miner. The work package consists of 3 strings
// result[0], 32 bytes hex encoded current block header pow-hash
// result[1], 32 bytes hex encoded seed hash used for DAG
// result[2], 32 bytes hex encoded boundary condition ("target"), 2^256/difficulty
func (api *PublicMinerAPI) GetWork() ([3]string, error) {
	if !api.e.IsMining() {
		return [3]string{}, errors.New("not mining now")
	}
	work, err := api.agent.GetWork()
	if err != nil {
		return work, fmt.Errorf("mining not ready: %v", err)
	}
	return work, nil
}

// SubmitHashrate can be used for remote miners to submit their hash rate. This enables the node to report the combined
// hash rate of all miners which submit work through this node. It accepts the miner hash rate and an identifier which
// must be unique between nodes.
func (api *PublicMinerAPI) SubmitHashrate(hashrate hexutil.Uint64, id common.Hash) bool {
	api.agent.SubmitHashrate(id, uint64(hashrate))
	return true
}

// PrivateMinerAPI provides private RPC methods to control the miner.
// These methods can be abused by external users and must be considered insecure for use by untrusted users.
type PrivateMinerAPI struct {
	e *Matrix
}

// NewPrivateMinerAPI create a new RPC service which controls the miner of this node.
func NewPrivateMinerAPI(e *Matrix) *PrivateMinerAPI {
	return &PrivateMinerAPI{e: e}
}

// Start the miner with the given number of threads. If threads is nil the number
// of workers started is equal to the number of logical CPUs that are usable by
// this process. If mining is already running, this method adjust the number of
// threads allowed to use.
func (api *PrivateMinerAPI) Start(threads *int) error {
	log.Info("设置开始CPU挖矿")
	// Set the number of threads if the seal engine supports it
	if threads == nil {
		threads = new(int)
	} else if *threads == 0 {
		*threads = -1 // Disable the miner from within
	}
	type threaded interface {
		SetThreads(threads int)
	}
	for version, engine := range api.e.engine {
		if th, ok := engine.(threaded); ok {
			log.Info("Updated mining threads", "engine version", version, "threads", *threads)
			th.SetThreads(*threads)
		}
	}

	// Start the cpu miner
	api.e.StartCupMining()
	return nil
}

// Stop the miner
func (api *PrivateMinerAPI) Stop() bool {
	log.Info("设置停止CPU挖矿")
	type threaded interface {
		SetThreads(threads int)
	}
	for version, engine := range api.e.engine {
		if th, ok := engine.(threaded); ok {
			log.Info("Updated mining threads", "engine version", version, "threads", -1)
			th.SetThreads(-1)
		}
	}
	api.e.StopCupMining()
	return true
}

func (api *PrivateMinerAPI) TestChangeRole(kind string, blocknum string, leader string) {
	/*
		var role common.RoleType
		switch kind {
		case "miner":
			role = common.RoleMiner
			log.INFO("TestChangeRole", "role", "common.RoleMiner")
		case "validator":
			role = common.RoleValidator
			log.INFO("TestChangeRole", "role", "common.RoleValidator")
		case "broadcast":
			role = common.RoleValidator
			log.INFO("TestChangeRole", "role", "common.RoleValidator")
		default:
			role = common.RoleDefault
			log.INFO("TestChangeRole", "role", "common.RoleDefault")
		}

		int, err := strconv.Atoi(blocknum)
		if err != nil {
			int = 1
		}

		var Leader common.Address
		data, err := hex.DecodeString(leader)
		if err != nil || leader == "" {
			log.ERROR("data DecodeString failed", "err", err)
			Leader = common.Address{}

		} else {
			Leader = common.BytesToAddress(data)
		}
		log.INFO("TestChangeRole", "Leader string", leader, "leader common.Address", Leader, "Address to Hex", Leader.Hex())
		mc.PublishEvent(mc.CA_RoleUpdated, &mc.RoleUpdatedMsg{Role: role, BlockNum: uint64(int), Leader: Leader})
		log.INFO("TestChangeRole", "Leader string", leader, "leader common.Address", Leader, "Address to Hex", common.HexToAddress(ca.Validatoraccountlist[1]))
		time.Sleep(time.Second)
		mc.PublishEvent(mc.Leader_LeaderChangeNotify, &mc.LeaderChangeNotify{true, common.HexToAddress(ca.Validatoraccountlist[1]), 1, 0})
	*/

	ans := mc.HD_MiningReqMsg{
		Header: &types.Header{
			Number: big.NewInt(100),
		},
	}
	data, err := json.Marshal(ans)
	if err != nil {
		log.INFO("SendToGroup", "Marshal failed err", err)
	}
	api.e.HD().SendNodeMsg(mc.HD_MiningReq, data, common.RoleValidator, nil)
}

func (api *PrivateMinerAPI) TestLocalMining(kind string, s string) {

	int, err := strconv.Atoi(s)
	if err != nil {
		int = 600000
	}
	time.Sleep(10 * time.Second)
	fmt.Println("开始发送挖矿请求消息")
	testHeader := &types.Header{
		ParentHash: common.BigToHash(big.NewInt(100)),
		Difficulty: big.NewInt(int64(int)),
		Number:     big.NewInt(331),
		Nonce:      types.EncodeNonce(8),
		Time:       big.NewInt(888),
		Coinbase:   common.BigToAddress(big.NewInt(123)),
		MixDigest:  common.BigToHash(big.NewInt(777)),
		Signatures: []common.Signature{common.BytesToSignature(common.BigToHash(big.NewInt(100)).Bytes())},
	}
	switch kind {
	case "vali_send":

		api.e.hd.SendNodeMsg(mc.HD_MiningReq, &mc.HD_MiningReqMsg{Header: testHeader}, common.RoleValidator, nil)
		log.INFO("发送给验证者", "data", mc.HD_MiningReqMsg{Header: testHeader})
	case "miner_send":

		api.e.hd.SendNodeMsg(mc.HD_MiningReq, &mc.HD_MiningReqMsg{Header: testHeader}, common.RoleMiner, nil)
		log.INFO("发送给矿工", "data", mc.HD_MiningReqMsg{Header: testHeader})
	case "signal_send":
		temp := "0x92e0fea9aba517398c2f0dd628f8cfc7e32ba984"
		nodes := []common.Address{common.HexToAddress(temp)}

		api.e.hd.SendNodeMsg(mc.HD_MiningReq, &mc.HD_MiningReqMsg{Header: testHeader}, common.RoleMiner, nodes)
		log.INFO("单点发送", "Data", nodes[0])
	case "normal_signal":
		mc.PublishEvent(mc.CA_RoleUpdated, &mc.RoleUpdatedMsg{Role: common.RoleMiner, BlockNum: 1})
		mc.PublishEvent(mc.HD_MiningReq, &mc.HD_MiningReqMsg{Header: testHeader})
		log.INFO("successfully normal ", "data", mc.HD_MiningReqMsg{Header: testHeader})

	default:
		//	log.INFO("successfully local", "data", mc.BlockData{Header: testHeader, Txs: &types.Transactions{}})
	}

}

func (api *PrivateMinerAPI) TestHeaderGen(kind string, s string) {
	num, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		num = 600000
	}
	time.Sleep(10 * time.Second)
	fmt.Println("开始发送挖矿请求消息")
	//testHeader := &types.Header{
	//	ParentHash: common.BigToHash(big.NewInt(100)),
	//	Difficulty: big.NewInt(int64(num)),
	//	Number:     big.NewInt(331),
	//	Nonce:      types.EncodeNonce(8),
	//	Time:       big.NewInt(888),
	//	Coinbase:   common.BigToAddress(big.NewInt(123)),
	//	MixDigest:  common.BigToHash(big.NewInt(777)),
	//	Signatures: []common.Signature{common.BytesToSignature(common.BigToHash(big.NewInt(100)).Bytes())},
	//}
	switch kind {
	case "gen":
		w := wizard.MakeWizard("MANSuperGenesis")

		hash := api.e.BlockChain().GetCurrentHash()
		currentNum := api.e.BlockChain().GetBlockByHash(hash).Number().Uint64()
		if num > currentNum+1 {
			log.Error("num is error", "current num:", currentNum)

		}
		w.MakeSuperGenesis(api.e.BlockChain(), api.e.chainDb, num, false)
		//mc.PublicEvent(mc.CA_RoleUpdated, &mc.RoleUpdatedMsg{Role: common.RoleValidator, BlockNum: 1})
		//mc.PublicEvent(mc.BlkVerify_VerifyConsensusOK, &mc.BlockVerifyConsensusOK{testHeader, nil, nil, nil})
		log.INFO("successfully gen superGenesis ", "MANSuperGenesis.", "nil")
	case "start":
		//type LeaderChangeNotify struct {
		//	ConsensusState bool //共识结果
		//	Leader         common.Address
		//	Number         uint64
		//	ReelectTurn    uint8
		//}
		//api.e.msgcenter.PostEvent(mc.CA_RoleUpdated, &mc.RoleUpdatedMsg{Role: common.RoleValidator, BlockNum: 1})
		////api.e.msgcenter.PostEvent(mc.Leader_LeaderChangeNotify, &mc.LeaderChangeNotify{true, nil, nil, nil})
		//log.INFO("successfully normal ", "start", mc.BlockVerifyConsensusOK{Header: testHeader})
	default:
		mc.PublishEvent(mc.CA_RoleUpdated, &mc.RoleUpdatedMsg{Role: common.RoleBroadcast, BlockNum: 1})
		//mc.PublishEvent(mc.BD_MiningReq, &mc.BlockGenor_BroadcastMiningReqMsg{BlockMainData: &mc.BlockData{Header: testHeader, Txs: &types.Transactions{}}})
		//log.INFO("successfully local", "data", mc.BlockData{Header: testHeader, Txs: &types.Transactions{}})
	}
}

// SetExtra sets the extra data string that is included when this miner mines a block.
func (api *PrivateMinerAPI) SetExtra(extra string) (bool, error) {
	if err := api.e.Miner().SetExtra([]byte(extra)); err != nil {
		return false, err
	}
	return true, nil
}

// SetGasPrice sets the minimum accepted gas price for the miner.
func (api *PrivateMinerAPI) SetGasPrice(gasPrice hexutil.Big) bool {
	api.e.lock.Lock()
	api.e.gasPrice = (*big.Int)(&gasPrice)
	api.e.lock.Unlock()

	//api.e.txPool.SetGasPrice((*big.Int)(&gasPrice)) //Y
	return true
}

// GetHashrate returns the current hashrate of the miner.
func (api *PrivateMinerAPI) GetHashrate() uint64 {
	return uint64(api.e.miner.HashRate())
}

// PrivateAdminAPI is the collection of Matrix full node-related APIs
// exposed over the private admin endpoint.
type PrivateAdminAPI struct {
	man *Matrix
}

// NewPrivateAdminAPI creates a new API definition for the full node private
// admin methods of the Matrix service.
func NewPrivateAdminAPI(man *Matrix) *PrivateAdminAPI {
	return &PrivateAdminAPI{man: man}
}

// ExportChain exports the current blockchain into a local file.
func (api *PrivateAdminAPI) ExportChain(file string) (bool, error) {
	// Make sure we can create the file to export into
	out, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return false, err
	}
	defer out.Close()

	var writer io.Writer = out
	if strings.HasSuffix(file, ".gz") {
		writer = gzip.NewWriter(writer)
		defer writer.(*gzip.Writer).Close()
	}

	// Export the blockchain
	if err := api.man.BlockChain().Export(writer); err != nil {
		return false, err
	}
	return true, nil
}

func hasAllBlocks(chain *core.BlockChain, bs []*types.Block) bool {
	for _, b := range bs {
		if !chain.HasBlock(b.Hash(), b.NumberU64()) {
			return false
		}
	}

	return true
}

// ImportChain imports a blockchain from a local file.
func (api *PrivateAdminAPI) ImportChain(file string) (bool, error) {
	// Make sure the can access the file to import
	in, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer in.Close()

	var reader io.Reader = in
	if strings.HasSuffix(file, ".gz") {
		if reader, err = gzip.NewReader(reader); err != nil {
			return false, err
		}
	}

	// Run actual the import in pre-configured batches
	stream := rlp.NewStream(reader, 0)

	blocks, index := make([]*types.Block, 0, 2500), 0
	for batch := 0; ; batch++ {
		// Load a batch of blocks from the input file
		for len(blocks) < cap(blocks) {
			block := new(types.Block)
			if err := stream.Decode(block); err == io.EOF {
				break
			} else if err != nil {
				return false, fmt.Errorf("block %d: failed to parse: %v", index, err)
			}
			blocks = append(blocks, block)
			index++
		}
		if len(blocks) == 0 {
			break
		}

		if hasAllBlocks(api.man.BlockChain(), blocks) {
			blocks = blocks[:0]
			continue
		}
		// Import the batch and reset the buffer
		if _, err := api.man.BlockChain().InsertChain(blocks, 0); err != nil {
			return false, fmt.Errorf("batch %d: failed to insert: %v", batch, err)
		}
		blocks = blocks[:0]
	}
	return true, nil
}

// PublicDebugAPI is the collection of Matrix full node APIs exposed
// over the public debugging endpoint.
type PublicDebugAPI struct {
	man *Matrix
}

// NewPublicDebugAPI creates a new API definition for the full node-
// related public debug methods of the Matrix service.
func NewPublicDebugAPI(man *Matrix) *PublicDebugAPI {
	return &PublicDebugAPI{man: man}
}

// DumpBlock retrieves the entire state of the database at a given block.
func (api *PublicDebugAPI) DumpBlock(blockNr rpc.BlockNumber, cointyp string, addr string) ([]state.CoinDump, error) {
	if cointyp != "" {
		cointyp = strings.ToUpper(cointyp)
	}
	var address common.Address
	if addr != "" {
		tmpaddress, err := base58.Base58DecodeToAddress(addr)
		if err != nil {
			return nil, err
		}
		address = tmpaddress
	}
	if blockNr == rpc.PendingBlockNumber {
		// If we're dumping the pending state, we need to request
		// both the pending block as well as the pending state from
		// the miner and operate on those
		_, stateDb := api.man.miner.Pending()
		return stateDb.RawDump(cointyp, address), nil
	}
	var block *types.Block
	if blockNr == rpc.LatestBlockNumber {
		block = api.man.blockchain.CurrentBlock()
	} else {
		block = api.man.blockchain.GetBlockByNumber(uint64(blockNr))
	}
	if block == nil {
		return nil, fmt.Errorf("block #%d not found", blockNr)
	}
	stateDb, err := api.man.BlockChain().StateAt(block.Root())
	if err != nil {
		return nil, err
	}
	return stateDb.RawDump(cointyp, address), nil
}

// DumpBlock retrieves the entire state of the database at a given block.
func (api *PublicDebugAPI) DumpBlockAccount(blockNr rpc.BlockNumber, address common.Address) (state.Dump, error) {
	//if blockNr == rpc.PendingBlockNumber {
	//	// If we're dumping the pending state, we need to request
	//	// both the pending block as well as the pending state from
	//	// the miner and operate on those
	//	_, stateDb := api.man.miner.Pending()
	//	return stateDb.RawDump(params.MAN_COIN, address), nil
	//}
	var block *types.Block
	if blockNr == rpc.LatestBlockNumber {
		block = api.man.blockchain.CurrentBlock()
	} else {
		block = api.man.blockchain.GetBlockByNumber(uint64(blockNr))
	}
	if block == nil {
		return state.Dump{}, fmt.Errorf("block #%d not found", blockNr)
	}
	stateDb, err := api.man.BlockChain().StateAt(block.Root())
	if err != nil {
		return state.Dump{}, err
	}
	return stateDb.RawDumpAcccount(params.MAN_COIN, address), nil
}

// PrivateDebugAPI is the collection of Matrix full node APIs exposed over
// the private debugging endpoint.
type PrivateDebugAPI struct {
	config *params.ChainConfig
	man    *Matrix
}

// NewPrivateDebugAPI creates a new API definition for the full node-related
// private debug methods of the Matrix service.
func NewPrivateDebugAPI(config *params.ChainConfig, man *Matrix) *PrivateDebugAPI {
	return &PrivateDebugAPI{config: config, man: man}
}

// Preimage is a debug API function that returns the preimage for a sha3 hash, if known.
func (api *PrivateDebugAPI) Preimage(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	if preimage := rawdb.ReadPreimage(api.man.ChainDb(), hash); preimage != nil {
		return preimage, nil
	}
	return nil, errors.New("unknown preimage")
}

// GetBadBLocks returns a list of the last 'bad blocks' that the client has seen on the network
// and returns them as a JSON list of block-hashes
func (api *PrivateDebugAPI) GetBadBlocks(ctx context.Context) ([]core.BadBlockArgs, error) {
	return api.man.BlockChain().BadBlocks()
}
func (api *PrivateDebugAPI) GetCommit(ctx context.Context) ([]common.CommitContext, error) {
	/*for _,v:=range common.PutCommit{
		fmt.Println(v)
	}*/
	return common.PutCommit, nil
}

// StorageRangeResult is the result of a debug_storageRangeAt API call.
type StorageRangeResult struct {
	Storage storageMap   `json:"storage"`
	NextKey *common.Hash `json:"nextKey"` // nil if Storage includes the last key in the trie.
}

type storageMap map[common.Hash]storageEntry

type storageEntry struct {
	Key   *common.Hash `json:"key"`
	Value common.Hash  `json:"value"`
}

// StorageRangeAt returns the storage at the given block height and transaction index.
func (api *PrivateDebugAPI) StorageRangeAt(ctx context.Context, blockHash common.Hash, txIndex int, contractAddress common.Address, keyStart hexutil.Bytes, maxResult int) (StorageRangeResult, error) {
	tx, _, statedb, err := api.computeTxEnv(blockHash, txIndex, 0)
	if err != nil {
		return StorageRangeResult{}, err
	}

	st := statedb.StorageTrie(tx.GetTxCurrency(), contractAddress)
	if st == nil {
		return StorageRangeResult{}, fmt.Errorf("account %x doesn't exist", contractAddress)
	}
	return storageRangeAt(st, keyStart, maxResult)
}

func storageRangeAt(st state.Trie, start []byte, maxResult int) (StorageRangeResult, error) {
	it := trie.NewIterator(st.NodeIterator(start))
	result := StorageRangeResult{Storage: storageMap{}}
	for i := 0; i < maxResult && it.Next(); i++ {
		_, content, _, err := rlp.Split(it.Value)
		if err != nil {
			return StorageRangeResult{}, err
		}
		e := storageEntry{Value: common.BytesToHash(content)}
		if preimage := st.GetKey(it.Key); preimage != nil {
			preimage := common.BytesToHash(preimage)
			e.Key = &preimage
		}
		result.Storage[common.BytesToHash(it.Key)] = e
	}
	// Add the 'next key' so clients can continue downloading.
	if it.Next() {
		next := common.BytesToHash(it.Key)
		result.NextKey = &next
	}
	return result, nil
}

// GetModifiedAccountsByumber returns all accounts that have changed between the
// two blocks specified. A change is defined as a difference in nonce, balance,
// code hash, or storage hash.
//
// With one parameter, returns the list of accounts modified in the specified block.
func (api *PrivateDebugAPI) GetModifiedAccountsByNumber(startNum uint64, endNum *uint64) ([]common.Address, error) {
	var startBlock, endBlock *types.Block

	startBlock = api.man.blockchain.GetBlockByNumber(startNum)
	if startBlock == nil {
		return nil, fmt.Errorf("start block %x not found", startNum)
	}

	if endNum == nil {
		endBlock = startBlock
		startBlock = api.man.blockchain.GetBlockByHash(startBlock.ParentHash())
		if startBlock == nil {
			return nil, fmt.Errorf("block %x has no parent", endBlock.Number())
		}
	} else {
		endBlock = api.man.blockchain.GetBlockByNumber(*endNum)
		if endBlock == nil {
			return nil, fmt.Errorf("end block %d not found", *endNum)
		}
	}
	return api.getModifiedAccounts(startBlock, endBlock)
}
func (api *PrivateDebugAPI) SetInterestPrintLevel(level uint8) error {

	util.SetLoglevel(level)
	return nil
}

// GetModifiedAccountsByHash returns all accounts that have changed between the
// two blocks specified. A change is defined as a difference in nonce, balance,
// code hash, or storage hash.
//
// With one parameter, returns the list of accounts modified in the specified block.
func (api *PrivateDebugAPI) GetModifiedAccountsByHash(startHash common.Hash, endHash *common.Hash) ([]common.Address, error) {
	var startBlock, endBlock *types.Block
	startBlock = api.man.blockchain.GetBlockByHash(startHash)
	if startBlock == nil {
		return nil, fmt.Errorf("start block %x not found", startHash)
	}

	if endHash == nil {
		endBlock = startBlock
		startBlock = api.man.blockchain.GetBlockByHash(startBlock.ParentHash())
		if startBlock == nil {
			return nil, fmt.Errorf("block %x has no parent", endBlock.Number())
		}
	} else {
		endBlock = api.man.blockchain.GetBlockByHash(*endHash)
		if endBlock == nil {
			return nil, fmt.Errorf("end block %x not found", *endHash)
		}
	}
	return api.getModifiedAccounts(startBlock, endBlock)
}

func (api *PrivateDebugAPI) getModifiedAccounts(startBlock, endBlock *types.Block) ([]common.Address, error) {
	if startBlock.Number().Uint64() >= endBlock.Number().Uint64() {
		return nil, fmt.Errorf("start block height (%d) must be less than end block height (%d)", startBlock.Number().Uint64(), endBlock.Number().Uint64())
	}

	stR := types.RlpHash(startBlock.Root())
	oldTrie, err := trie.NewSecure(stR, trie.NewDatabase(api.man.chainDb), 0)
	if err != nil {
		return nil, err
	}
	enR := types.RlpHash(endBlock.Root())
	newTrie, err := trie.NewSecure(enR, trie.NewDatabase(api.man.chainDb), 0)
	if err != nil {
		return nil, err
	}

	diff, _ := trie.NewDifferenceIterator(oldTrie.NodeIterator([]byte{}), newTrie.NodeIterator([]byte{}))
	iter := trie.NewIterator(diff)

	var dirty []common.Address
	for iter.Next() {
		key := newTrie.GetKey(iter.Key)
		if key == nil {
			return nil, fmt.Errorf("no preimage found for hash %x", iter.Key)
		}
		dirty = append(dirty, common.BytesToAddress(key))
	}
	return dirty, nil
}
