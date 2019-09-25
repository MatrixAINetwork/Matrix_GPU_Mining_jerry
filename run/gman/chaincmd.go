// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MatrixAINetwork/go-matrix/params"

	"github.com/MatrixAINetwork/go-matrix/accounts/keystore"
	"github.com/MatrixAINetwork/go-matrix/crypto/aes"
	"github.com/MatrixAINetwork/go-matrix/man/wizard"

	"encoding/base64"

	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/console"
	"github.com/MatrixAINetwork/go-matrix/core"
	"github.com/MatrixAINetwork/go-matrix/core/state"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/crypto"
	"github.com/MatrixAINetwork/go-matrix/event"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/man/downloader"
	"github.com/MatrixAINetwork/go-matrix/mandb"
	"github.com/MatrixAINetwork/go-matrix/mc"
	"github.com/MatrixAINetwork/go-matrix/run/utils"
	"github.com/MatrixAINetwork/go-matrix/trie"
	"github.com/syndtr/goleveldb/leveldb/util"
	"gopkg.in/urfave/cli.v1"
)

var (
	initCommand = cli.Command{
		Action:    utils.MigrateFlags(initGenesis),
		Name:      "init",
		Usage:     "Bootstrap and initialize a new genesis block",
		ArgsUsage: "<genesisPath>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.LightModeFlag,
			utils.GetGenesisFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The init command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}
	importCommand = cli.Command{
		Action:    utils.MigrateFlags(importChain),
		Name:      "import",
		Usage:     "Import a blockchain file",
		ArgsUsage: "<filename> (<filename 2> ... <filename N>) ",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.CacheFlag,
			utils.LightModeFlag,
			utils.GCModeFlag,
			utils.CacheDatabaseFlag,
			utils.CacheGCFlag,
			utils.DbTableSizeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The import command imports blocks from an RLP-encoded form. The form can be one file
with several RLP-encoded blocks, or several files can be used.

If only one file is used, import error will result in failure. If several files are used,
processing will proceed even if an individual RLP-file import failure occurs.`,
	}
	exportCommand = cli.Command{
		Action:    utils.MigrateFlags(exportChain),
		Name:      "export",
		Usage:     "Export blockchain into file",
		ArgsUsage: "<filename> [<blockNumFirst> <blockNumLast>]",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.CacheFlag,
			utils.LightModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
Requires a first argument of the file to write to.
Optional second and third arguments control the first and
last block to write. In this mode, the file will be appended
if already existing.`,
	}
	importPreimagesCommand = cli.Command{
		Action:    utils.MigrateFlags(importPreimages),
		Name:      "import-preimages",
		Usage:     "Import the preimage database from an RLP stream",
		ArgsUsage: "<datafile>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.CacheFlag,
			utils.LightModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
	The import-preimages command imports hash preimages from an RLP encoded stream.`,
	}
	exportPreimagesCommand = cli.Command{
		Action:    utils.MigrateFlags(exportPreimages),
		Name:      "export-preimages",
		Usage:     "Export the preimage database into an RLP stream",
		ArgsUsage: "<dumpfile>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.CacheFlag,
			utils.LightModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The export-preimages command export hash preimages to an RLP encoded stream`,
	}
	copydbCommand = cli.Command{
		Action:    utils.MigrateFlags(copyDb),
		Name:      "copydb",
		Usage:     "Create a local chain from a target chaindata folder",
		ArgsUsage: "<sourceChaindataDir>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.CacheFlag,
			utils.SyncModeFlag,
			utils.FakePoWFlag,
			utils.TestnetFlag,
			utils.RinkebyFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The first argument must be the directory containing the blockchain to download from`,
	}
	removedbCommand = cli.Command{
		Action:    utils.MigrateFlags(removeDB),
		Name:      "removedb",
		Usage:     "Remove blockchain and state databases",
		ArgsUsage: " ",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.LightModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
Remove blockchain and state databases`,
	}
	dumpCommand = cli.Command{
		Action:    utils.MigrateFlags(dump),
		Name:      "dump",
		Usage:     "Dump a specific block from storage",
		ArgsUsage: "[<blockHash> | <blockNum>]...",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.CacheFlag,
			utils.LightModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The arguments are interpreted as block numbers or hashes.
Use "matrix dump 0" to dump the genesis block.`,
	}
	CommitCommand = cli.Command{
		Action:      utils.MigrateFlags(getCommit),
		Name:        "commit",
		Usage:       "Commit history ,include version submitter and commit",
		ArgsUsage:   "",
		Flags:       []cli.Flag{},
		Category:    "commit commands",
		Description: "get commit history",
	}
	rollbackCommand = cli.Command{
		Action:    utils.MigrateFlags(rollback),
		Name:      "rollback",
		Usage:     "Bootstrap and rollback a new super block",
		ArgsUsage: "<genesisPath>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.LightModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}

	importSupBlockCommand = cli.Command{
		Action:    utils.MigrateFlags(importSupBlock),
		Name:      "importSuperBlock",
		Usage:     "Bootstrap and rollback a new super block",
		ArgsUsage: "<genesisPath>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.LightModeFlag,
			utils.GCModeFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}
	genBlockCommand = cli.Command{
		Action:    utils.MigrateFlags(genblock),
		Name:      "genblock",
		Usage:     "Bootstrap and rollback a new super block",
		ArgsUsage: "<genesisPath> blockNum",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.SuperBlockElectGenFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}
	genBlockRootsCommand = cli.Command{
		Action:    utils.MigrateFlags(genBlockRoots),
		Name:      "genblockroots",
		Usage:     "Bootstrap and rollback a new super block",
		ArgsUsage: "<genesisPath> blockNum",
		Flags: []cli.Flag{
			utils.DataDirFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}
	signCommand = cli.Command{
		Action:    utils.MigrateFlags(signBlock),
		Name:      "signblock",
		Usage:     "sign a new super block",
		ArgsUsage: "<datafile>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}

	signSuperBlockCommand = cli.Command{
		Action:    utils.MigrateFlags(signSuperBlock),
		Name:      "signsuperblock",
		Usage:     "sign a new super block",
		ArgsUsage: "",
		Flags:     []cli.Flag{},
		Category:  "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}

	signVersionCommand = cli.Command{
		Action:    utils.MigrateFlags(signVersion),
		Name:      "signversion",
		Usage:     "sign  version",
		ArgsUsage: "<genesisPath> blockNum",
		Flags:     []cli.Flag{},
		Category:  "BLOCKCHAIN COMMANDS",
		Description: `
The rollback command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}

	AesEncryptCommand = cli.Command{
		Action:    utils.MigrateFlags(aesEncrypt),
		Name:      "aes",
		Usage:     "encrypt  a file",
		ArgsUsage: "",
		Flags: []cli.Flag{
			utils.AesInputFlag,
			utils.AesOutputFlag,
		},
		Category:    "aes commands",
		Description: "aes commit",
	}

	AiTestCommand = cli.Command{
		Action:      utils.MigrateFlags(aiTest),
		Name:        "aitest",
		Usage:       "test ai digging",
		ArgsUsage:   "",
		Flags:       []cli.Flag{},
		Category:    "aitest commands",
		Description: "aitest commit",
	}
)

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func initGenesis(ctx *cli.Context) error {
	// Make sure we have a valid genesis JSON
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply path to genesis JSON file")
	}
	genesis, err := core.DefaultGenesis(genesisPath)
	if err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	mergeOutputPath := ctx.GlobalString(utils.GetGenesisFlag.Name)
	if len(genesisPath) != 0 {
		out, _ := json.MarshalIndent(genesis, "", "  ")
		if err := ioutil.WriteFile(mergeOutputPath, out, 0644); err != nil {
			fmt.Errorf("Failed to save genesis file", "err=%v", err)
		}
	}

	//genesis = core.DefaultGenesisToEthGensis(genesis1, genesis)

	// Open an initialise both full and light databases
	stack := makeFullNode(ctx)
	for _, name := range []string{"chaindata", "lightchaindata"} {
		chaindb, err := stack.OpenDatabase(name, 0, 0)
		if err != nil {
			utils.Fatalf("Failed to open database: %v", err)
		}
		_, hash, err := core.SetupGenesisBlock(chaindb, genesis)
		if err != nil {
			utils.Fatalf("Failed to write genesis block: %v", err)
		}
		log.Info("Successfully wrote genesis state", "database", name, "hash", hash)
	}
	return nil
}

func importChain(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	stack := makeFullNode(ctx)
	chain, chainDb := utils.MakeChain(ctx, stack)
	defer chainDb.Close()

	// Start periodically gathering memory profiles
	var peakMemAlloc, peakMemSys uint64
	go func() {
		stats := new(runtime.MemStats)
		for {
			runtime.ReadMemStats(stats)
			if atomic.LoadUint64(&peakMemAlloc) < stats.Alloc {
				atomic.StoreUint64(&peakMemAlloc, stats.Alloc)
			}
			if atomic.LoadUint64(&peakMemSys) < stats.Sys {
				atomic.StoreUint64(&peakMemSys, stats.Sys)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	// Import the chain
	start := time.Now()

	if len(ctx.Args()) == 1 {
		if err := utils.ImportChain(chain, ctx.Args().First()); err != nil {
			log.Error("Import error", "err", err)
		}
	} else {
		for _, arg := range ctx.Args() {
			if err := utils.ImportChain(chain, arg); err != nil {
				log.Error("Import error", "file", arg, "err", err)
			}
		}
	}
	chain.Stop()
	fmt.Printf("Import done in %v.\n\n", time.Since(start))

	// Output pre-compaction stats mostly to see the import trashing
	db := chainDb.(*mandb.LDBDatabase)

	stats, err := db.LDB().GetProperty("leveldb.stats")
	if err != nil {
		utils.Fatalf("Failed to read database stats: %v", err)
	}
	fmt.Println(stats)

	ioStats, err := db.LDB().GetProperty("leveldb.iostats")
	if err != nil {
		utils.Fatalf("Failed to read database iostats: %v", err)
	}
	fmt.Println(ioStats)

	fmt.Printf("Trie cache misses:  %d\n", trie.CacheMisses())
	fmt.Printf("Trie cache unloads: %d\n\n", trie.CacheUnloads())

	// Print the memory statistics used by the importing
	mem := new(runtime.MemStats)
	runtime.ReadMemStats(mem)

	fmt.Printf("Object memory: %.3f MB current, %.3f MB peak\n", float64(mem.Alloc)/1024/1024, float64(atomic.LoadUint64(&peakMemAlloc))/1024/1024)
	fmt.Printf("System memory: %.3f MB current, %.3f MB peak\n", float64(mem.Sys)/1024/1024, float64(atomic.LoadUint64(&peakMemSys))/1024/1024)
	fmt.Printf("Allocations:   %.3f million\n", float64(mem.Mallocs)/1000000)
	fmt.Printf("GC pause:      %v\n\n", time.Duration(mem.PauseTotalNs))

	if ctx.GlobalIsSet(utils.NoCompactionFlag.Name) {
		return nil
	}

	// Compact the entire database to more accurately measure disk io and print the stats
	start = time.Now()
	fmt.Println("Compacting entire database...")
	if err = db.LDB().CompactRange(util.Range{}); err != nil {
		utils.Fatalf("Compaction failed: %v", err)
	}
	fmt.Printf("Compaction done in %v.\n\n", time.Since(start))

	stats, err = db.LDB().GetProperty("leveldb.stats")
	if err != nil {
		utils.Fatalf("Failed to read database stats: %v", err)
	}
	fmt.Println(stats)

	ioStats, err = db.LDB().GetProperty("leveldb.iostats")
	if err != nil {
		utils.Fatalf("Failed to read database iostats: %v", err)
	}
	fmt.Println(ioStats)

	return nil
}

func exportChain(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	stack := makeFullNode(ctx)
	chain, _ := utils.MakeChain(ctx, stack)
	start := time.Now()

	var err error
	fp := ctx.Args().First()
	if len(ctx.Args()) < 3 {
		err = utils.ExportChain(chain, fp)
	} else {
		// This can be improved to allow for numbers larger than 9223372036854775807
		first, ferr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
		last, lerr := strconv.ParseInt(ctx.Args().Get(2), 10, 64)
		if ferr != nil || lerr != nil {
			utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
		}
		if first < 0 || last < 0 {
			utils.Fatalf("Export error: block number must be greater than 0\n")
		}
		err = utils.ExportAppendChain(chain, fp, uint64(first), uint64(last))
	}

	if err != nil {
		utils.Fatalf("Export error: %v\n", err)
	}
	fmt.Printf("Export done in %v\n", time.Since(start))
	return nil
}

// importPreimages imports preimage data from the specified file.
func importPreimages(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	stack := makeFullNode(ctx)
	diskdb := utils.MakeChainDatabase(ctx, stack).(*mandb.LDBDatabase)

	start := time.Now()
	if err := utils.ImportPreimages(diskdb, ctx.Args().First()); err != nil {
		utils.Fatalf("Export error: %v\n", err)
	}
	fmt.Printf("Export done in %v\n", time.Since(start))
	return nil
}

// exportPreimages dumps the preimage data to specified json file in streaming way.
func exportPreimages(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	stack := makeFullNode(ctx)
	diskdb := utils.MakeChainDatabase(ctx, stack).(*mandb.LDBDatabase)

	start := time.Now()
	if err := utils.ExportPreimages(diskdb, ctx.Args().First()); err != nil {
		utils.Fatalf("Export error: %v\n", err)
	}
	fmt.Printf("Export done in %v\n", time.Since(start))
	return nil
}

func copyDb(ctx *cli.Context) error {
	// Ensure we have a source chain directory to copy
	if len(ctx.Args()) != 1 {
		utils.Fatalf("Source chaindata directory path argument missing")
	}
	// Initialize a new chain for the running node to sync into
	stack := makeFullNode(ctx)
	chain, chainDb := utils.MakeChain(ctx, stack)

	syncmode := *utils.GlobalTextMarshaler(ctx, utils.SyncModeFlag.Name).(*downloader.SyncMode)
	dl := downloader.New(syncmode, chainDb, new(event.TypeMux), chain, nil, nil, nil)

	// Create a source peer to satisfy downloader requests from
	db, err := mandb.NewLDBDatabase(ctx.Args().First(), ctx.GlobalInt(utils.CacheFlag.Name), 256, 2)
	if err != nil {
		return err
	}
	hc, err := core.NewHeaderChain(db, chain.Config(), func() bool { return false })
	if err != nil {
		return err
	}
	currentHeader := hc.CurrentHeader()
	peer := downloader.NewFakePeer("local", db, hc, dl)
	if err = dl.RegisterPeer("local", 63, peer); err != nil {
		return err
	}
	// Synchronise with the simulated peer
	start := time.Now()
	sbs, err := chain.GetSuperBlockNum()
	if nil != err {
		return err
	}
	if err = dl.Synchronise("local", currentHeader.Hash(), hc.GetTd(currentHeader.Hash(), currentHeader.Number.Uint64()), currentHeader.SuperBlockSeq(), sbs, syncmode); err != nil {
		return err
	}
	for dl.Synchronising() {
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Printf("Database copy done in %v\n", time.Since(start))

	// Compact the entire database to remove any sync overhead
	start = time.Now()
	fmt.Println("Compacting entire database...")
	if err = chainDb.(*mandb.LDBDatabase).LDB().CompactRange(util.Range{}); err != nil {
		utils.Fatalf("Compaction failed: %v", err)
	}
	fmt.Printf("Compaction done in %v.\n\n", time.Since(start))

	return nil
}

func removeDB(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)

	for _, name := range []string{"chaindata", "lightchaindata"} {
		// Ensure the database exists in the first place
		logger := log.New("database", name)

		dbdir := stack.ResolvePath(name)
		if !common.FileExist(dbdir) {
			logger.Info("Database doesn't exist, skipping", "path", dbdir)
			continue
		}
		// Confirm removal and execute
		fmt.Println(dbdir)
		confirm, err := console.Stdin.PromptConfirm("Remove this database?")
		switch {
		case err != nil:
			utils.Fatalf("%v", err)
		case !confirm:
			logger.Warn("Database deletion aborted")
		default:
			start := time.Now()
			os.RemoveAll(dbdir)
			logger.Info("Database successfully deleted", "elapsed", common.PrettyDuration(time.Since(start)))
		}
	}
	return nil
}

func dump(ctx *cli.Context) error {
	stack := makeFullNode(ctx)
	chain, chainDb := utils.MakeChain(ctx, stack)
	for _, arg := range ctx.Args() {
		var block *types.Block
		if hashish(arg) {
			block = chain.GetBlockByHash(common.HexToHash(arg))
		} else {
			num, _ := strconv.Atoi(arg)
			block = chain.GetBlockByNumber(uint64(num))
		}
		if block == nil {
			fmt.Println("{}")
			utils.Fatalf("block not found")
		} else {
			state, err := state.NewStateDBManage(block.Root(), chainDb, state.NewDatabase(chainDb))
			if err != nil {
				utils.Fatalf("could not create new state: %v", err)
			}
			fmt.Printf("%s\n", state.Dump(params.MAN_COIN, common.Address{})) //
		}
	}
	chainDb.Close()
	return nil
}

// hashish returns true for strings that look like hashes.
func hashish(x string) bool {
	_, err := strconv.Atoi(x)
	return err != nil
}
func getCommit(ctx *cli.Context) error {
	for _, v := range common.PutCommit {
		fmt.Println(v)
	}
	return nil
}

func importSupBlock(ctx *cli.Context) error {
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply path to genesis JSON file")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
		return err
	}
	defer file.Close()

	matrixGenesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(matrixGenesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
		return err
	}

	stack := makeFullNode(ctx)
	chain, _ := utils.MakeChain(ctx, stack)
	if chain == nil {
		utils.Fatalf("make chain err")
		return errors.New("make chain err")
	}

	//core.ManGenesisToEthGensis(matrixGenesis, genesis)
	if _, err := chain.InsertSuperBlock(matrixGenesis, false); err != nil {
		utils.Fatalf("insert super block err(%v)", err)
		return err
	}

	chain.Stop()

	return nil
}

func rollback(ctx *cli.Context) error {
	Snum := ctx.Args().First()
	if len(Snum) == 0 {
		utils.Fatalf("Must supply num")
		return nil
	}
	num, err := strconv.ParseUint(Snum, 10, 64)
	if err != nil {
		utils.Fatalf("conver supply num error%v", err)
		return nil
	}
	stack := makeFullNode(ctx)
	chain, _ := utils.MakeChain(ctx, stack)
	chain.SetHead(num)
	return nil
}

func genblock(ctx *cli.Context) error {
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply path to genesis JSON file")
	}
	Snum := ctx.Args().Get(1)
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply num")
	}
	num, err := strconv.ParseUint(Snum, 10, 64)
	if err != nil {
		utils.Fatalf("conver supply num error%v", err)
	}
	stack := makeFullNode(ctx)
	chain, chaindb := utils.MakeChain(ctx, stack)
	w := wizard.MakeWizard(genesisPath)

	hash := chain.GetCurrentHash()
	currentNum := chain.GetBlockByHash(hash).Number().Uint64()
	if num > currentNum+1 {
		log.Error("num is error", "current num:", currentNum)
		return errors.New("num is error")

	}
	var electFlag bool
	if ctx.GlobalBool(utils.SuperBlockElectGenFlag.Name) {
		electFlag = true
	}
	w.MakeSuperGenesis(chain, chaindb, num, electFlag)
	//w.ManageSuperGenesis(chainDb)
	return nil
}

func genBlockRoots(ctx *cli.Context) error {
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("keyfile must be given as argument")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	matrixGenesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(matrixGenesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	stack, _ := makeConfigNode(ctx)
	chain, chainDB := utils.MakeChain(ctx, stack)
	if chain == nil {
		utils.Fatalf("make chain err")
	}

	parent := chain.GetHeaderByHash(matrixGenesis.ParentHash)
	if nil == parent {
		utils.Fatalf("get parent header err")
	}
	genesis := new(core.Genesis)
	//core.ManGenesisToEthGensis(matrixGenesis, genesis)
	genesis = matrixGenesis
	superBlock := genesis.GenSuperBlock(parent, chainDB, state.NewDatabase(chainDB), chain.Config())
	if nil == superBlock {
		utils.Fatalf("genesis super block err")
	}

	matrixGenesis.Roots = make([]common.CoinRoot, len(superBlock.Root()))
	copy(matrixGenesis.Roots, superBlock.Root())
	matrixGenesis.Sharding = make([]common.Coinbyte, len(superBlock.Sharding()))
	copy(matrixGenesis.Sharding, superBlock.Sharding())
	//matrixGenesis.TxHash = superBlock.TxHash()
	pathSplit := strings.Split(genesisPath, ".json")
	out, _ := json.MarshalIndent(matrixGenesis, "", "  ")
	if err := ioutil.WriteFile(pathSplit[0]+"Roots.json", out, 0644); err != nil {
		utils.Fatalf("Failed to save genesis file, err = %v", err)
	}
	fmt.Println("Exported sign  block to ", pathSplit[0]+"Roots.json")
	return nil
}

func signSuperBlock(ctx *cli.Context) error {
	if len(ctx.Args()) < 2 {
		utils.Fatalf("This command requires 2 arguments.")
	}
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("keyfile must be given as argument")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	privateKey := ctx.Args().Get(1)
	if len(privateKey) == 0 {
		utils.Fatalf("private key must >0")
	}

	matrixGenesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(matrixGenesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}
	//	// get block hash
	ECDSPrivateKey, err := crypto.HexToECDSA(privateKey)
	if nil != err {
		utils.Fatalf("input private key error")
	}
	block := matrixGenesis.ToSuperBlock()
	blockHash := block.HashNoSigns()
	fmt.Println("blockhash:", blockHash.Hex())
	signBytes, err := crypto.Sign(blockHash.Bytes(), ECDSPrivateKey)
	if err != nil {
		utils.Fatalf("Unlocked account: %v", err)
	}

	sign := common.BytesToSignature(signBytes)
	//matrixGenesis.TxHash = superBlock.TxHash()
	matrixGenesis.Signatures = append(matrixGenesis.Signatures, sign)
	pathSplit := strings.Split(genesisPath, ".json")
	out, _ := json.MarshalIndent(matrixGenesis, "", "  ")
	if err := ioutil.WriteFile(pathSplit[0]+"Signed.json", out, 0644); err != nil {
		utils.Fatalf("Failed to save genesis file, err = %v", err)
	}
	fmt.Println("Exported sign  block to", pathSplit[0]+"Signed.json")
	return nil
}

func signBlock(ctx *cli.Context) error {
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("keyfile must be given as argument")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	matrixGenesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(matrixGenesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	stack, _ := makeConfigNode(ctx)
	chain, chainDB := utils.MakeChain(ctx, stack)
	if chain == nil {
		utils.Fatalf("make chain err")
	}

	parent := chain.GetHeaderByHash(matrixGenesis.ParentHash)
	if nil == parent {
		utils.Fatalf("get parent header err")
	}
	genesis := new(core.Genesis)
	//core.ManGenesisToEthGensis(matrixGenesis, genesis)
	genesis = matrixGenesis
	superBlock := genesis.GenSuperBlock(parent, chainDB, state.NewDatabase(chainDB), chain.Config())
	if nil == superBlock {
		utils.Fatalf("genesis super block err")
	}
	// get block hash
	blockHash := superBlock.HashNoSigns()
	passwordList, err := utils.GetSignPassword(ctx)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	passPhrase := getPassPhrase("", false, 0, passwordList)
	if len(stack.AccountManager().Wallets()) <= 0 {
		utils.Fatalf("can't find wallet")
	}
	wallet := stack.AccountManager().Wallets()[0]
	if len(wallet.Accounts()) <= 0 {
		utils.Fatalf("can't find account")
	}
	account := wallet.Accounts()[0]
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	if err := ks.Unlock(account, passPhrase); err != nil {
		utils.Fatalf("unlock account failed")
	}
	fmt.Println("blockhash:", blockHash.Hex())
	signBytes, err := ks.SignHash(account, blockHash.Bytes())
	if err != nil {
		utils.Fatalf("Unlocked account: %v", err)
	}

	sign := common.BytesToSignature(signBytes)
	matrixGenesis.Roots = make([]common.CoinRoot, len(superBlock.Root()))
	copy(matrixGenesis.Roots, superBlock.Root())
	//matrixGenesis.TxHash = superBlock.TxHash()
	matrixGenesis.Signatures = append(genesis.Signatures, sign)
	pathSplit := strings.Split(genesisPath, ".json")
	out, _ := json.MarshalIndent(matrixGenesis, "", "  ")
	if err := ioutil.WriteFile(pathSplit[0]+"Signed.json", out, 0644); err != nil {
		utils.Fatalf("Failed to save genesis file, err = %v", err)
	}
	fmt.Println("Exported sign  block to ", pathSplit[0]+"Signed.json")
	return nil
}

func signTestModeVersion(ctx *cli.Context) error {
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("keyfile must be given as argument")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	//genesis := new(core.Genesis1)
	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	passwordList, err := utils.GetSignPassword(ctx)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	passphrase := getPassPhrase("", false, 0, passwordList)

	stack, _ := makeConfigNode(ctx)
	accounts := stack.AccountManager()
	if nil == accounts {
		utils.Fatalf("no accounts")
		return nil
	}

	wallets := accounts.Wallets()
	if 0 == len(wallets) {
		utils.Fatalf("no wallet ")
		return nil
	}
	wallet := wallets[0]

	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	err = ks.Unlock(wallet.Accounts()[0], passphrase)
	if err != nil {
		utils.Fatalf("Unlocked account %v", err)
		return nil
	}
	sign, err := ks.SignHashVersionWithPass(wallet.Accounts()[0], passphrase, common.BytesToHash([]byte(genesis.Version)).Bytes())
	if err != nil {
		utils.Fatalf("Unlocked account %v", err)
		return nil
	}
	temp := common.BytesToSignature(sign)
	genesis.VersionSignatures = append(genesis.VersionSignatures, temp)
	account, err := crypto.VerifySignWithVersion(common.BytesToHash([]byte(genesis.Version)).Bytes(), sign)
	//fmt.Printf("Address: {%x}\n", acct.Address)
	if !account.Equal(wallet.Accounts()[0].Address) {
		utils.Fatalf("sign verion error")
		return nil
	}
	pathSplit := strings.Split(genesisPath, ".json")
	out, _ := json.MarshalIndent(genesis, "", "  ")
	if err := ioutil.WriteFile(pathSplit[0]+"VersionSigned.json", out, 0644); err != nil {
		fmt.Errorf("Failed to save genesis file", "err=%v", err)
		return nil
	}
	fmt.Println("Exported sign  version to ", pathSplit[0]+"VersionSigned.json")
	return nil
}

func signVersion(ctx *cli.Context) error {
	if len(ctx.Args()) < 2 {
		utils.Fatalf("This command requires 2 arguments.")
	}
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("keyfile must be given as argument")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()
	privateKey := ctx.Args().Get(1)
	if len(privateKey) == 0 {
		utils.Fatalf("private key must >0")
	}

	//genesis := new(core.Genesis1)
	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	ECDSPrivateKey, err := crypto.HexToECDSA(privateKey)
	if nil != err {
		utils.Fatalf("input private key error")
	}
	signBytes, err := crypto.Sign(common.BytesToHash([]byte(genesis.Version)).Bytes(), ECDSPrivateKey)
	if err != nil {
		utils.Fatalf("Unlocked account: %v", err)
	}
	temp := common.BytesToSignature(signBytes)
	pathSplit := strings.Split(genesisPath, ".json")
	out, _ := json.MarshalIndent(temp, "", "  ")
	if err := ioutil.WriteFile(pathSplit[0]+"VersionSigned.json", out, 0644); err != nil {
		fmt.Errorf("Failed to save genesis file", "err=%v", err)
		return nil
	}
	fmt.Println("Exported sign  version to ", pathSplit[0]+"VersionSigned.json")
	return nil
}
func aesEncrypt(ctx *cli.Context) error {
	Inpath := ctx.GlobalString(utils.AesInputFlag.Name)
	fmt.Println("Your input is", Inpath)
	fmt.Println("Your password's length must be between 8 and 16 characters, and should contain numbers, uppercase letters (A-Z), lowercase letters (a-z) and special characters")
	JsonParse := NewJsonStruct()
	fileValue := []mc.EntrustInfo{}
	JsonParse.Load(Inpath, &fileValue)

	dataV, err := json.Marshal(fileValue)
	if err != nil {
		return errors.New("Marshalling on texts faild")
	}
	entrustPassword, err := ReadDecryptPassword(utils.Twice, ctx)
	if err != nil {
		return err
	}

	xpass, err := aes.AesEncrypt(dataV, []byte(entrustPassword))
	if err != nil {
		return errors.New("Encryption Failed")
	}
	pass64 := base64.StdEncoding.EncodeToString(xpass)
	//fmt.Println("加密后", pass64)

	outPath := ctx.GlobalString(utils.AesOutputFlag.Name)
	//写入文件
	err = ioutil.WriteFile(outPath, []byte(pass64), 0666)
	if err == nil {
		fmt.Println("Write into " + outPath + " successfully")
	}

	return nil
}

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("Failed to read the common profile, err", err, "file", filename)
		os.Exit(-1)
		return
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		fmt.Println("Failed to obtain data from the common profile, err", err, "filename", filename)
		os.Exit(-1)
		return
	}
}
