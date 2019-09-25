// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

// Package utils contains internal helper functions for go-matrix commands.
package utils

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/MatrixAINetwork/go-matrix/base58"

	"encoding/json"

	"github.com/MatrixAINetwork/go-matrix/accounts"
	"github.com/MatrixAINetwork/go-matrix/accounts/keystore"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/common/fdlimit"
	"github.com/MatrixAINetwork/go-matrix/consensus"
	"github.com/MatrixAINetwork/go-matrix/consensus/amhash"
	"github.com/MatrixAINetwork/go-matrix/consensus/clique"
	"github.com/MatrixAINetwork/go-matrix/consensus/manash"
	"github.com/MatrixAINetwork/go-matrix/consensus/mtxdpos"
	"github.com/MatrixAINetwork/go-matrix/console"
	"github.com/MatrixAINetwork/go-matrix/core"
	"github.com/MatrixAINetwork/go-matrix/core/state"
	"github.com/MatrixAINetwork/go-matrix/core/vm"
	"github.com/MatrixAINetwork/go-matrix/crypto"
	"github.com/MatrixAINetwork/go-matrix/dashboard"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/man"
	"github.com/MatrixAINetwork/go-matrix/man/downloader"
	"github.com/MatrixAINetwork/go-matrix/man/gasprice"
	"github.com/MatrixAINetwork/go-matrix/mandb"
	"github.com/MatrixAINetwork/go-matrix/manstats"
	"github.com/MatrixAINetwork/go-matrix/metrics"
	"github.com/MatrixAINetwork/go-matrix/p2p"
	"github.com/MatrixAINetwork/go-matrix/p2p/discover"
	"github.com/MatrixAINetwork/go-matrix/p2p/nat"
	"github.com/MatrixAINetwork/go-matrix/p2p/netutil"
	"github.com/MatrixAINetwork/go-matrix/params"
	"github.com/MatrixAINetwork/go-matrix/params/manversion"
	"github.com/MatrixAINetwork/go-matrix/pod"
	"gopkg.in/urfave/cli.v1"
)

var (
	CommandHelpTemplate = `{{.cmd.Name}}{{if .cmd.Subcommands}} command{{end}}{{if .cmd.Flags}} [command options]{{end}} [arguments...]
{{if .cmd.Description}}{{.cmd.Description}}
{{end}}{{if .cmd.Subcommands}}
SUBCOMMANDS:
	{{range .cmd.Subcommands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
	{{end}}{{end}}{{if .categorizedFlags}}
{{range $idx, $categorized := .categorizedFlags}}{{$categorized.Name}} OPTIONS:
{{range $categorized.Flags}}{{"\t"}}{{.}}
{{end}}
{{end}}{{end}}`
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

	cli.CommandHelpTemplate = CommandHelpTemplate
}

// NewApp creates an app with sane defaults.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	//app.Authors = nil
	app.Email = ""
	app.Version = params.Version
	if len(gitCommit) >= 8 {
		app.Version += "-" + gitCommit[:8]
	}
	app.Usage = usage
	return app
}

// These are all the command line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate command definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.

var (
	// General settings
	DataDirFlag = DirectoryFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DirectoryString{pod.DefaultDataDir()},
	}
	KeyStoreDirFlag = DirectoryFlag{
		Name:  "keystore",
		Usage: "Directory for the keystore (default = inside the datadir)",
	}
	NoUSBFlag = cli.BoolFlag{
		Name:  "nousb",
		Usage: "Disables monitoring for and managing USB hardware wallets",
	}
	NetworkIdFlag = cli.Uint64Flag{
		Name:  "networkid",
		Usage: "Network identifier (integer, 1=Frontier, 2=Morden (disused), 3=Ropsten, 4=Rinkeby)",
		Value: man.DefaultConfig.NetworkId,
	}
	TestnetFlag = cli.BoolFlag{
		Name:  "testnet",
		Usage: "Ropsten network: pre-configured proof-of-work test network",
	}
	RinkebyFlag = cli.BoolFlag{
		Name:  "rinkeby",
		Usage: "Rinkeby network: pre-configured proof-of-authority test network",
	}
	DeveloperFlag = cli.BoolFlag{
		Name:  "dev",
		Usage: "Ephemeral proof-of-authority network with a pre-funded developer account, mining enabled",
	}
	DeveloperPeriodFlag = cli.IntFlag{
		Name:  "dev.period",
		Usage: "Block period to use in developer mode (0 = mine only if transaction pending)",
	}
	IdentityFlag = cli.StringFlag{
		Name:  "identity",
		Usage: "Custom node name",
	}
	DocRootFlag = DirectoryFlag{
		Name:  "docroot",
		Usage: "Document Root for HTTPClient file scheme",
		Value: DirectoryString{homeDir()},
	}
	FastSyncFlag = cli.BoolFlag{
		Name:  "fast",
		Usage: "Enable fast syncing through state downloads (replaced by --syncmode)",
	}
	LightModeFlag = cli.BoolFlag{
		Name:  "light",
		Usage: "Enable light client mode (replaced by --syncmode)",
	}
	GetGenesisFlag = cli.StringFlag{
		Name:  "getGenesis",
		Usage: "Output the actual configuration used",
	}
	defaultSyncMode = man.DefaultConfig.SyncMode
	SyncModeFlag    = TextMarshalerFlag{
		Name:  "syncmode",
		Usage: `Blockchain sync mode ("fast", "full", or "light")`,
		Value: &defaultSyncMode,
	}
	GCModeFlag = cli.StringFlag{
		Name:  "gcmode",
		Usage: `Blockchain garbage collection mode ("full", "archive")`,
		Value: "archive",
	}
	DbTableSizeFlag = cli.IntFlag{
		Name:  "dbsize",
		Usage: "db store size ",
		Value: 2,
	}
	LightServFlag = cli.IntFlag{
		Name:  "lightserv",
		Usage: "Maximum percentage of time allowed for serving LES requests (0-90)",
		Value: 0,
	}
	LightPeersFlag = cli.IntFlag{
		Name:  "lightpeers",
		Usage: "Maximum number of LES client peers",
		Value: man.DefaultConfig.LightPeers,
	}
	LightKDFFlag = cli.BoolFlag{
		Name:  "lightkdf",
		Usage: "Reduce key-derivation RAM & CPU usage at some expense of KDF strength",
	}
	AesInputFlag = cli.StringFlag{
		Name:  "aesin",
		Usage: "aes 输入",
	}
	AesOutputFlag = cli.StringFlag{
		Name:  "aesout",
		Usage: "aes 输出",
	}
	// Dashboard settings
	DashboardEnabledFlag = cli.BoolFlag{
		Name:  "dashboard",
		Usage: "Enable the dashboard",
	}
	DashboardAddrFlag = cli.StringFlag{
		Name:  "dashboard.addr",
		Usage: "Dashboard listening interface",
		Value: dashboard.DefaultConfig.Host,
	}
	DashboardPortFlag = cli.IntFlag{
		Name:  "dashboard.host",
		Usage: "Dashboard listening port",
		Value: dashboard.DefaultConfig.Port,
	}
	DashboardRefreshFlag = cli.DurationFlag{
		Name:  "dashboard.refresh",
		Usage: "Dashboard metrics collection refresh rate",
		Value: dashboard.DefaultConfig.Refresh,
	}
	// Manash settings
	ManashCacheDirFlag = DirectoryFlag{
		Name:  "manash.cachedir",
		Usage: "Directory to store the manash verification caches (default = inside the datadir)",
	}
	ManashCachesInMemoryFlag = cli.IntFlag{
		Name:  "manash.cachesinmem",
		Usage: "Number of recent manash caches to keep in memory (16MB each)",
		Value: man.DefaultConfig.Manash.CachesInMem,
	}
	ManashCachesOnDiskFlag = cli.IntFlag{
		Name:  "manash.cachesondisk",
		Usage: "Number of recent manash caches to keep on disk (16MB each)",
		Value: man.DefaultConfig.Manash.CachesOnDisk,
	}
	ManashDatasetDirFlag = DirectoryFlag{
		Name:  "manash.dagdir",
		Usage: "Directory to store the manash mining DAGs (default = inside home folder)",
		Value: DirectoryString{man.DefaultConfig.Manash.DatasetDir},
	}
	ManashDatasetsInMemoryFlag = cli.IntFlag{
		Name:  "manash.dagsinmem",
		Usage: "Number of recent manash mining DAGs to keep in memory (1+GB each)",
		Value: man.DefaultConfig.Manash.DatasetsInMem,
	}
	ManashDatasetsOnDiskFlag = cli.IntFlag{
		Name:  "manash.dagsondisk",
		Usage: "Number of recent manash mining DAGs to keep on disk (1+GB each)",
		Value: man.DefaultConfig.Manash.DatasetsOnDisk,
	}
	// Transaction pool settings
	TxPoolNoLocalsFlag = cli.BoolFlag{
		Name:  "txpool.nolocals",
		Usage: "Disables price exemptions for locally submitted transactions",
	}
	//TxPoolJournalFlag = cli.StringFlag{ //Y
	//	Name:  "txpool.journal",
	//	Usage: "Disk journal for local transaction to survive node restarts",
	//	Value: core.DefaultTxPoolConfig.Journal,
	//}
	//TxPoolRejournalFlag = cli.DurationFlag{
	//	Name:  "txpool.rejournal",
	//	Usage: "Time interval to regenerate the local transaction journal",
	//	Value: core.DefaultTxPoolConfig.Rejournal,
	//}
	TxPoolPriceLimitFlag = cli.Uint64Flag{
		Name:  "txpool.pricelimit",
		Usage: "Minimum gas price limit to enforce for acceptance into the pool",
		Value: man.DefaultConfig.TxPool.PriceLimit,
	}
	//TxPoolPriceBumpFlag = cli.Uint64Flag{ //Y
	//	Name:  "txpool.pricebump",
	//	Usage: "Price bump percentage to replace an already existing transaction",
	//	Value: eth.DefaultConfig.TxPool.PriceBump,
	//}
	TxPoolAccountSlotsFlag = cli.Uint64Flag{
		Name:  "txpool.accountslots",
		Usage: "Minimum number of executable transaction slots guaranteed per account",
		Value: man.DefaultConfig.TxPool.AccountSlots,
	}
	TxPoolGlobalSlotsFlag = cli.Uint64Flag{
		Name:  "txpool.globalslots",
		Usage: "Maximum number of executable transaction slots for all accounts",
		Value: man.DefaultConfig.TxPool.GlobalSlots,
	}
	TxPoolAccountQueueFlag = cli.Uint64Flag{
		Name:  "txpool.accountqueue",
		Usage: "Maximum number of non-executable transaction slots permitted per account",
		Value: man.DefaultConfig.TxPool.AccountQueue,
	}
	TxPoolGlobalQueueFlag = cli.Uint64Flag{
		Name:  "txpool.globalqueue",
		Usage: "Maximum number of non-executable transaction slots for all accounts",
		Value: man.DefaultConfig.TxPool.GlobalQueue,
	}
	//TxPoolLifetimeFlag = cli.DurationFlag{ //Y
	//	Name:  "txpool.lifetime",
	//	Usage: "Maximum amount of time non-executable transaction are queued",
	//	Value: eth.DefaultConfig.TxPool.Lifetime,
	//}
	// Performance tuning settings
	CacheFlag = cli.IntFlag{
		Name:  "cache",
		Usage: "Megabytes of memory allocated to internal caching",
		Value: 1024,
	}
	CacheDatabaseFlag = cli.IntFlag{
		Name:  "cache.database",
		Usage: "Percentage of cache memory allowance to use for database io",
		Value: 75,
	}
	CacheGCFlag = cli.IntFlag{
		Name:  "cache.gc",
		Usage: "Percentage of cache memory allowance to use for trie pruning",
		Value: 25,
	}
	TrieCacheGenFlag = cli.IntFlag{
		Name:  "trie-cache-gens",
		Usage: "Number of trie node generations to keep in memory",
		Value: int(state.MaxTrieCacheGen),
	}
	// Miner settings
	MiningEnabledFlag = cli.BoolFlag{
		Name:  "mine",
		Usage: "Enable mining",
	}
	MinerThreadsFlag = cli.IntFlag{
		Name:  "minerthreads",
		Usage: "Number of CPU threads to use for mining",
		Value: runtime.NumCPU(),
	}
	TargetGasLimitFlag = cli.Uint64Flag{
		Name:  "targetgaslimit",
		Usage: "Target gas limit sets the artificial target gas floor for the blocks to mine",
		Value: params.GenesisGasLimit,
	}
	ManerbaseFlag = cli.StringFlag{
		Name:  "manbase",
		Usage: "Public address for block mining rewards (default = first account created)",
		Value: "0",
	}
	GasPriceFlag = BigFlag{
		Name:  "gasprice",
		Usage: "Minimal gas price to accept for mining a transactions",
		Value: man.DefaultConfig.GasPrice,
	}
	ExtraDataFlag = cli.StringFlag{
		Name:  "extradata",
		Usage: "Block extra data set by the miner (default = client version)",
	}
	TestLocalMiningFlag = cli.StringFlag{
		Name:  "testlocalmining",
		Usage: "print a string",
	}
	SuperBlockElectGenFlag = cli.BoolFlag{
		Name:  "electflag",
		Usage: " super block elect gen",
	}
	TestChangeRoleFlag = cli.StringFlag{
		Name:  "testchangerole",
		Usage: "change role",
	}
	GetCommitFlag = cli.StringFlag{
		Name:  "testgetcommit",
		Usage: "get commit",
	}
	// Account settings
	UnlockedAccountFlag = cli.StringFlag{
		Name:  "unlock",
		Usage: "Comma separated list of accounts to unlock",
		Value: "",
	}
	PasswordFileFlag = cli.StringFlag{
		Name:  "password",
		Usage: "Password file to use for non-interactive password input",
		Value: "",
	}
	ManAddressFlag = cli.StringFlag{
		Name:  "manAddress",
		Usage: "deposit user signature account.",
		Value: "",
	}
	AccountPasswordFileFlag = cli.StringFlag{
		Name:  "entrust",
		Usage: "Password file to entrustment transaction",
		Value: "",
	}
	TestEntrustFlag = cli.StringFlag{
		Name:  "testmode",
		Usage: "",
		Value: "",
	}
	VMEnableDebugFlag = cli.BoolFlag{
		Name:  "vmdebug",
		Usage: "Record information useful for VM and contract debugging",
	}
	// Logging and debug settings
	ManStatsURLFlag = cli.StringFlag{
		Name:  "manstats",
		Usage: "Reporting URL of a manstats service (nodename:secret@host:port)",
	}
	MetricsEnabledFlag = cli.BoolFlag{
		Name:  metrics.MetricsEnabledFlag,
		Usage: "Enable metrics collection and reporting",
	}
	FakePoWFlag = cli.BoolFlag{
		Name:  "fakepow",
		Usage: "Disables proof-of-work verification",
	}
	NoCompactionFlag = cli.BoolFlag{
		Name:  "nocompaction",
		Usage: "Disables db compaction after import",
	}
	// RPC settings
	RPCEnabledFlag = cli.BoolFlag{
		Name:  "rpc",
		Usage: "Enable the HTTP-RPC server",
	}
	RPCListenAddrFlag = cli.StringFlag{
		Name:  "rpcaddr",
		Usage: "HTTP-RPC server listening interface",
		Value: pod.DefaultHTTPHost,
	}
	RPCPortFlag = cli.IntFlag{
		Name:  "rpcport",
		Usage: "HTTP-RPC server listening port",
		Value: pod.DefaultHTTPPort,
	}
	RPCCORSDomainFlag = cli.StringFlag{
		Name:  "rpccorsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests (browser enforced)",
		Value: "",
	}
	RPCVirtualHostsFlag = cli.StringFlag{
		Name:  "rpcvhosts",
		Usage: "Comma separated list of virtual hostnames from which to accept requests (server enforced). Accepts '*' wildcard.",
		Value: strings.Join(pod.DefaultConfig.HTTPVirtualHosts, ","),
	}
	RPCApiFlag = cli.StringFlag{
		Name:  "rpcapi",
		Usage: "API's offered over the HTTP-RPC interface",
		Value: "",
	}
	IPCDisabledFlag = cli.BoolFlag{
		Name:  "ipcdisable",
		Usage: "Disable the IPC-RPC server",
	}
	IPCPathFlag = DirectoryFlag{
		Name:  "ipcpath",
		Usage: "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
	}
	WSEnabledFlag = cli.BoolFlag{
		Name:  "ws",
		Usage: "Enable the WS-RPC server",
	}
	WSListenAddrFlag = cli.StringFlag{
		Name:  "wsaddr",
		Usage: "WS-RPC server listening interface",
		Value: pod.DefaultWSHost,
	}
	WSPortFlag = cli.IntFlag{
		Name:  "wsport",
		Usage: "WS-RPC server listening port",
		Value: pod.DefaultWSPort,
	}
	WSApiFlag = cli.StringFlag{
		Name:  "wsapi",
		Usage: "API's offered over the WS-RPC interface",
		Value: "",
	}
	WSAllowedOriginsFlag = cli.StringFlag{
		Name:  "wsorigins",
		Usage: "Origins from which to accept websockets requests",
		Value: "",
	}
	ExecFlag = cli.StringFlag{
		Name:  "exec",
		Usage: "Execute JavaScript statement",
	}
	PreloadJSFlag = cli.StringFlag{
		Name:  "preload",
		Usage: "Comma separated list of JavaScript files to preload into the console",
	}

	// Network Settings
	MaxPeersFlag = cli.IntFlag{
		Name:  "maxpeers",
		Usage: "Maximum number of network peers (network disabled if set to 0)",
		Value: 25,
	}
	MaxPendingPeersFlag = cli.IntFlag{
		Name:  "maxpendpeers",
		Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
		Value: 0,
	}
	ListenPortFlag = cli.IntFlag{
		Name:  "port",
		Usage: "Network listening port",
		Value: 50505,
	}
	BootnodesFlag = cli.StringFlag{
		Name:  "bootnodes",
		Usage: "Comma separated enode URLs for P2P discovery bootstrap (set v4+v5 instead for light servers)",
		Value: "",
	}
	BootnodesV4Flag = cli.StringFlag{
		Name:  "bootnodesv4",
		Usage: "Comma separated enode URLs for P2P v4 discovery bootstrap (light server, full nodes)",
		Value: "",
	}
	BootnodesV5Flag = cli.StringFlag{
		Name:  "bootnodesv5",
		Usage: "Comma separated enode URLs for P2P v5 discovery bootstrap (light server, light nodes)",
		Value: "",
	}
	NodeKeyFileFlag = cli.StringFlag{
		Name:  "nodekey",
		Usage: "P2P node key file",
	}
	NodeKeyHexFlag = cli.StringFlag{
		Name:  "nodekeyhex",
		Usage: "P2P node key as hex (for testing)",
	}
	NATFlag = cli.StringFlag{
		Name:  "nat",
		Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		Value: "any",
	}
	NoDiscoverFlag = cli.BoolFlag{
		Name:  "nodiscover",
		Usage: "Disables the peer discovery mechanism (manual peer addition)",
	}
	DiscoveryV5Flag = cli.BoolFlag{
		Name:  "v5disc",
		Usage: "Enables the experimental RLPx V5 (Topic Discovery) mechanism",
	}
	NetrestrictFlag = cli.StringFlag{
		Name:  "netrestrict",
		Usage: "Restricts network communication to the given IP networks (CIDR masks)",
	}

	// ATM the url is left to the user and deployment to
	JSpathFlag = cli.StringFlag{
		Name:  "jspath",
		Usage: "JavaScript root path for `loadScript`",
		Value: ".",
	}

	// Gas price oracle settings
	GpoBlocksFlag = cli.IntFlag{
		Name:  "gpoblocks",
		Usage: "Number of recent blocks to check for gas prices",
		Value: man.DefaultConfig.GPO.Blocks,
	}
	GpoPercentileFlag = cli.IntFlag{
		Name:  "gpopercentile",
		Usage: "Suggested gas price is the given percentile of a set of recent transaction gas prices",
		Value: man.DefaultConfig.GPO.Percentile,
	}
	SynSnapshootNumFlg = cli.Uint64Flag{
		Name:  "snapnumber",
		Usage: "snapshoot sync block number",
		Value: man.SnapshootNumber,
	}
	SynSnapshootHashFlg = cli.StringFlag{
		Name:  "snaphash",
		Usage: "snapshoot sync block hash",
		Value: man.SnapshootHash,
	}
	SnapModeFlg = cli.IntFlag{
		Name:  "snapFlag",
		Usage: "snapFlag 0:from broadcast, 1:local",
		Value: man.SnaploadFromLocal,
	}
	SaveSnapStartFlg = cli.Uint64Flag{
		Name:  "snapstart",
		Usage: "snapshoot start switch",
		Value: man.SaveSnapStart,
	}
	SaveSnapPeriodFlg = cli.Uint64Flag{
		Name:  "snapperiod",
		Usage: "snapshoot save period,default 300",
		Value: man.SaveSnapPeriod,
	}
	LessDiskEnabledFlag = cli.BoolFlag{
		Name:  "lessdisk",
		Usage: "Enable the Less Disk Server",
	}
)

// MakeDataDir retrieves the currently requested data directory, terminating
// if none (or the empty string) is specified. If the node is starting a testnet,
// the a subdirectory of the specified datadir will be used.
func MakeDataDir(ctx *cli.Context) string {
	if path := ctx.GlobalString(DataDirFlag.Name); path != "" {
		if ctx.GlobalBool(TestnetFlag.Name) {
			return filepath.Join(path, "testnet")
		}
		if ctx.GlobalBool(RinkebyFlag.Name) {
			return filepath.Join(path, "rinkeby")
		}
		return path
	}
	Fatalf("Cannot determine default data directory, please set manually (--datadir)")
	return ""
}

// setNodeKey creates a node key from set command line flags, either loading it
// from a file or as a specified hex value. If neither flags were provided, this
// method returns nil and an emphemeral key is to be generated.
func setNodeKey(ctx *cli.Context, cfg *p2p.Config) {
	var (
		hex  = ctx.GlobalString(NodeKeyHexFlag.Name)
		file = ctx.GlobalString(NodeKeyFileFlag.Name)
		key  *ecdsa.PrivateKey
		err  error
	)
	switch {
	case file != "" && hex != "":
		Fatalf("Options %q and %q are mutually exclusive", NodeKeyFileFlag.Name, NodeKeyHexFlag.Name)
	case file != "":
		if key, err = crypto.LoadECDSA(file); err != nil {
			Fatalf("Option %q: %v", NodeKeyFileFlag.Name, err)
		}
		cfg.PrivateKey = key
	case hex != "":
		if key, err = crypto.HexToECDSA(hex); err != nil {
			Fatalf("Option %q: %v", NodeKeyHexFlag.Name, err)
		}
		cfg.PrivateKey = key
	}
}

// setNodeUserIdent creates the user identifier from CLI flags.
func setNodeUserIdent(ctx *cli.Context, cfg *pod.Config) {
	if identity := ctx.GlobalString(IdentityFlag.Name); len(identity) > 0 {
		cfg.UserIdent = identity
	}
}

// setBootstrapNodes creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrapNodes(ctx *cli.Context, cfg *p2p.Config) {
	urls := params.MainnetBootnodes
	switch {
	case ctx.GlobalIsSet(BootnodesFlag.Name) || ctx.GlobalIsSet(BootnodesV4Flag.Name):
		if ctx.GlobalIsSet(BootnodesV4Flag.Name) {
			urls = strings.Split(ctx.GlobalString(BootnodesV4Flag.Name), ",")
		} else {
			urls = strings.Split(ctx.GlobalString(BootnodesFlag.Name), ",")
		}
	case ctx.GlobalBool(TestnetFlag.Name):
		urls = params.TestnetBootnodes
	case ctx.GlobalBool(RinkebyFlag.Name):
		urls = params.RinkebyBootnodes
	case cfg.BootstrapNodes != nil:
		return // already set, don't apply defaults.
	}

	cfg.BootstrapNodes = make([]*discover.Node, 0, len(urls))
	for _, url := range urls {
		node, err := discover.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
	}
}

// setBootstrapNodesV5 creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
//func setBootstrapNodesV5(ctx *cli.Context, cfg *p2p.Config) {
//	urls := params.DiscoveryV5Bootnodes
//	switch {
//	case ctx.GlobalIsSet(BootnodesFlag.Name) || ctx.GlobalIsSet(BootnodesV5Flag.Name):
//		if ctx.GlobalIsSet(BootnodesV5Flag.Name) {
//			urls = strings.Split(ctx.GlobalString(BootnodesV5Flag.Name), ",")
//		} else {
//			urls = strings.Split(ctx.GlobalString(BootnodesFlag.Name), ",")
//		}
//	case ctx.GlobalBool(RinkebyFlag.Name):
//		urls = params.RinkebyBootnodes
//	//case cfg.BootstrapNodesV5 != nil:
//	//	return // already set, don't apply defaults.
//	}
//
//	cfg.BootstrapNodesV5 = make([]*discv5.Node, 0, len(urls))
//	for _, url := range urls {
//		node, err := discv5.ParseNode(url)
//		if err != nil {
//			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
//			continue
//		}
//		cfg.BootstrapNodesV5 = append(cfg.BootstrapNodesV5, node)
//	}
//}

// setListenAddress creates a TCP listening address string from set command
// line flags.
func setListenAddress(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(ListenPortFlag.Name) {
		cfg.ListenAddr = fmt.Sprintf(":%d", ctx.GlobalInt(ListenPortFlag.Name))
	}
}

// setNAT creates a port mapper from command line flags.
func setNAT(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(NATFlag.Name) {
		natif, err := nat.Parse(ctx.GlobalString(NATFlag.Name))
		if err != nil {
			Fatalf("Option %s: %v", NATFlag.Name, err)
		}
		cfg.NAT = natif
	}
}

// splitAndTrim splits input separated by a comma
// and trims excessive white space from the substrings.
func splitAndTrim(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

// setHTTP creates the HTTP RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setHTTP(ctx *cli.Context, cfg *pod.Config) {
	if ctx.GlobalBool(RPCEnabledFlag.Name) && cfg.HTTPHost == "" {
		cfg.HTTPHost = "127.0.0.1"
		if ctx.GlobalIsSet(RPCListenAddrFlag.Name) {
			cfg.HTTPHost = ctx.GlobalString(RPCListenAddrFlag.Name)
		}
	}

	if ctx.GlobalIsSet(RPCPortFlag.Name) {
		cfg.HTTPPort = ctx.GlobalInt(RPCPortFlag.Name)
	}
	if ctx.GlobalIsSet(RPCCORSDomainFlag.Name) {
		cfg.HTTPCors = splitAndTrim(ctx.GlobalString(RPCCORSDomainFlag.Name))
	}
	if ctx.GlobalIsSet(RPCApiFlag.Name) {
		cfg.HTTPModules = splitAndTrim(ctx.GlobalString(RPCApiFlag.Name))
	}
	if ctx.GlobalIsSet(RPCVirtualHostsFlag.Name) {
		cfg.HTTPVirtualHosts = splitAndTrim(ctx.GlobalString(RPCVirtualHostsFlag.Name))
	}
}

// setWS creates the WebSocket RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setWS(ctx *cli.Context, cfg *pod.Config) {
	if ctx.GlobalBool(WSEnabledFlag.Name) && cfg.WSHost == "" {
		cfg.WSHost = "127.0.0.1"
		if ctx.GlobalIsSet(WSListenAddrFlag.Name) {
			cfg.WSHost = ctx.GlobalString(WSListenAddrFlag.Name)
		}
	}

	if ctx.GlobalIsSet(WSPortFlag.Name) {
		cfg.WSPort = ctx.GlobalInt(WSPortFlag.Name)
	}
	if ctx.GlobalIsSet(WSAllowedOriginsFlag.Name) {
		cfg.WSOrigins = splitAndTrim(ctx.GlobalString(WSAllowedOriginsFlag.Name))
	}
	if ctx.GlobalIsSet(WSApiFlag.Name) {
		cfg.WSModules = splitAndTrim(ctx.GlobalString(WSApiFlag.Name))
	}
}

// setIPC creates an IPC path configuration from the set command line flags,
// returning an empty string if IPC was explicitly disabled, or the set path.
func setIPC(ctx *cli.Context, cfg *pod.Config) {
	checkExclusive(ctx, IPCDisabledFlag, IPCPathFlag)
	switch {
	case ctx.GlobalBool(IPCDisabledFlag.Name):
		cfg.IPCPath = ""
	case ctx.GlobalIsSet(IPCPathFlag.Name):
		cfg.IPCPath = ctx.GlobalString(IPCPathFlag.Name)
	}
}

// makeDatabaseHandles raises out the number of allowed file handles per process
// for Gman and returns half of the allowance to assign to the database.
func makeDatabaseHandles() int {
	limit, err := fdlimit.Current()
	if err != nil {
		Fatalf("Failed to retrieve file descriptor allowance: %v", err)
	}
	if limit < 2048 {
		if err := fdlimit.Raise(2048); err != nil {
			Fatalf("Failed to raise file descriptor allowance: %v", err)
		}
	}
	if limit > 2048 { // cap database file descriptors even if more is available
		limit = 2048
	}
	return limit / 2 // Leave half for networking and other stuff
}

// MakeAddress converts an account specified directly as a hex encoded string or
// a key index in the key store to an internal account representation.
func MakeAddress(ks *keystore.KeyStore, account string) (accounts.Account, error) {
	// If the specified account is a valid address, return it
	if common.IsHexAddress(account) {
		return accounts.Account{Address: common.HexToAddress(account)}, nil
	}
	// Otherwise try to interpret the account as a keystore index
	index, err := strconv.Atoi(account)
	if err != nil || index < 0 {
		return accounts.Account{}, fmt.Errorf("invalid account address or index %q", account)
	}
	log.Warn("-------------------------------------------------------------------")
	log.Warn("Referring to accounts by order in the keystore folder is dangerous!")
	log.Warn("This functionality is deprecated and will be removed in the future!")
	log.Warn("Please use explicit addresses! (can search via `gman account list`)")
	log.Warn("-------------------------------------------------------------------")

	accs := ks.Accounts()
	if len(accs) <= index {
		return accounts.Account{}, fmt.Errorf("index %d higher than number of accounts %d", index, len(accs))
	}
	return accs[index], nil
}

// setManerbase retrieves the manbase either from the directly specified
// command line flags or from the keystore if CLI indexed.
func setManerbase(ctx *cli.Context, ks *keystore.KeyStore, cfg *man.Config) {
	if ctx.GlobalIsSet(ManerbaseFlag.Name) {
		account, err := MakeAddress(ks, ctx.GlobalString(ManerbaseFlag.Name))
		if err != nil {
			Fatalf("Option %q: %v", ManerbaseFlag.Name, err)
		}
		cfg.Manerbase = account.Address
	}
}

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("读取通用配置文件失败 err", err, "file", filename)

		return
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		fmt.Println("通用配置文件数据获取失败 err", err)
		return
	}
}

type EntrustPassword struct {
	Password map[common.Address]string
}

// MakePasswordList reads password lines from the file specified by the global --password flag.
func MakePasswordList(ctx *cli.Context) []string {
	path := ctx.GlobalString(PasswordFileFlag.Name)
	if path == "" {
		return nil
	}
	text, err := ioutil.ReadFile(path)
	if err != nil {
		Fatalf("Failed to read password file: %v", err)
	}
	lines := strings.Split(string(text), "\n")
	// Sanitise DOS line endings.
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

// GetSignPassword returns password specified by the global --testmode flag.
func GetSignPassword(ctx *cli.Context) ([]string, error) {
	var password string
	if password = ctx.GlobalString(TestEntrustFlag.Name); password != "" {
		return []string{password}, nil
	}
	var err error
	password, err = GetPassword(Once)
	if err != nil {
		return nil, err
	}
	return []string{password}, nil
}

const (
	Once = iota
	Twice
)

func GetPassword(inputTimes int) (string, error) {
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		return "", fmt.Errorf("Failed to read passphrase: %v", err)
	}
	if inputTimes == Once {
		return password, nil
	}
	confirm, err := console.Stdin.PromptPassword("Repeat passphrase: ")
	if err != nil {
		return "", fmt.Errorf("Failed to read passphrase confirmation: %v", err)
	}
	if password != confirm {
		return "", fmt.Errorf("Passphrases do not match")
	}
	return password, nil
}

func IsValidChar(aim byte) bool {
	if aim >= 33 && aim <= 126 {
		return true
	}
	return false
}
func CheckPassword(password string) bool {
	flagLowerChar := false
	flagUpperChar := false
	flagNum := false
	flagSpecialChar := false
	for _, v := range password {
		if IsValidChar(byte(v)) == false {
			fmt.Println("There are unsupported characters in your password. Please try again")
			return false
		}
		switch {
		case v >= 'a' && v <= 'z':
			flagLowerChar = true
		case v >= 'A' && v <= 'Z':
			flagUpperChar = true
		case v >= '0' && v <= '9':
			flagNum = true
		default:
			flagSpecialChar = true
		}

	}

	if flagSpecialChar == false {
		fmt.Println("Your password doesn't contain special characters. Please try again")
		return false
	}
	if flagNum == false {
		fmt.Println("Your password doesn't contain numbers. Please try again")
		return false
	}
	if flagUpperChar == false {
		fmt.Println("Your password doesn't contain uppercase letters. Please try again")
		return false
	}
	if flagLowerChar == false {
		fmt.Println("Your password doesn't contain lowercase letters. Please try again")
		return false
	}

	if len(password) > 16 {
		fmt.Println("Your password's length exceeds 16 characters. Please try again")
		return false
	}
	if len(password) < 8 {
		fmt.Println("Your password's length is less than 8 characters. Please try again")
		return false
	}
	return true
}

func SetP2PConfig(ctx *cli.Context, cfg *p2p.Config) {
	setNodeKey(ctx, cfg)
	setNAT(ctx, cfg)
	setListenAddress(ctx, cfg)
	setBootstrapNodes(ctx, cfg)
	//setBootstrapNodesV5(ctx, cfg)

	lightClient := ctx.GlobalBool(LightModeFlag.Name) || ctx.GlobalString(SyncModeFlag.Name) == "light"
	lightServer := ctx.GlobalInt(LightServFlag.Name) != 0
	lightPeers := ctx.GlobalInt(LightPeersFlag.Name)

	if ctx.GlobalIsSet(MaxPeersFlag.Name) {
		cfg.MaxPeers = ctx.GlobalInt(MaxPeersFlag.Name)
		if lightServer && !ctx.GlobalIsSet(LightPeersFlag.Name) {
			cfg.MaxPeers += lightPeers
		}
	} else {
		if lightServer {
			cfg.MaxPeers += lightPeers
		}
		if lightClient && ctx.GlobalIsSet(LightPeersFlag.Name) && cfg.MaxPeers < lightPeers {
			cfg.MaxPeers = lightPeers
		}
	}
	if !(lightClient || lightServer) {
		lightPeers = 0
	}
	manPeers := cfg.MaxPeers - lightPeers
	if lightClient {
		manPeers = 0
	}
	log.Info("Maximum peer count", "MAN", manPeers, "LES", lightPeers, "total", cfg.MaxPeers)

	if ctx.GlobalIsSet(MaxPendingPeersFlag.Name) {
		cfg.MaxPendingPeers = ctx.GlobalInt(MaxPendingPeersFlag.Name)
	}
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.NetWorkId = ctx.GlobalUint64(NetworkIdFlag.Name)
	}
	if ctx.GlobalIsSet(NoDiscoverFlag.Name) || lightClient {
		cfg.NoDiscovery = true
	}
	if manAddr := ctx.GlobalString(ManAddressFlag.Name); manAddr != "" {
		innerAddr, err := base58.Base58DecodeToAddress(manAddr)
		if err == nil {
			cfg.ManAddress = innerAddr
		}
	}

	// if we're running a light client or server, force enable the v5 peer discovery
	// unless it is explicitly disabled with --nodiscover note that explicitly specifying
	// --v5disc overrides --nodiscover, in which case the later only disables v4 discovery
	//forceV5Discovery := (lightClient || lightServer) && !ctx.GlobalBool(NoDiscoverFlag.Name)
	//if ctx.GlobalIsSet(DiscoveryV5Flag.Name) {
	//	cfg.DiscoveryV5 = ctx.GlobalBool(DiscoveryV5Flag.Name)
	//} else if forceV5Discovery {
	//	cfg.DiscoveryV5 = true
	//}

	if netrestrict := ctx.GlobalString(NetrestrictFlag.Name); netrestrict != "" {
		list, err := netutil.ParseNetlist(netrestrict)
		if err != nil {
			Fatalf("Option %q: %v", NetrestrictFlag.Name, err)
		}
		cfg.NetRestrict = list
	}

	if ctx.GlobalBool(DeveloperFlag.Name) {
		// --dev mode can't use p2p networking.
		cfg.MaxPeers = 0
		cfg.ListenAddr = ":0"
		cfg.NoDiscovery = true
		//cfg.DiscoveryV5 = false
	}
}

// SetNodeConfig applies node-related command line flags to the config.
func SetNodeConfig(ctx *cli.Context, cfg *pod.Config) {
	SetP2PConfig(ctx, &cfg.P2P)
	setIPC(ctx, cfg)
	setHTTP(ctx, cfg)
	setWS(ctx, cfg)
	setNodeUserIdent(ctx, cfg)

	switch {
	case ctx.GlobalIsSet(DataDirFlag.Name):
		cfg.DataDir = ctx.GlobalString(DataDirFlag.Name)
	case ctx.GlobalBool(DeveloperFlag.Name):
		cfg.DataDir = "" // unless explicitly requested, use memory databases
	case ctx.GlobalBool(TestnetFlag.Name):
		cfg.DataDir = filepath.Join(pod.DefaultDataDir(), "testnet")
	case ctx.GlobalBool(RinkebyFlag.Name):
		cfg.DataDir = filepath.Join(pod.DefaultDataDir(), "rinkeby")
	}

	if ctx.GlobalIsSet(KeyStoreDirFlag.Name) {
		cfg.KeyStoreDir = ctx.GlobalString(KeyStoreDirFlag.Name)
	}
	if ctx.GlobalIsSet(LightKDFFlag.Name) {
		cfg.UseLightweightKDF = ctx.GlobalBool(LightKDFFlag.Name)
	}
	if ctx.GlobalIsSet(NoUSBFlag.Name) {
		cfg.NoUSB = ctx.GlobalBool(NoUSBFlag.Name)
	}
	if ctx.GlobalIsSet(LessDiskEnabledFlag.Name) {
		cfg.LessDisk = ctx.GlobalBool(LessDiskEnabledFlag.Name)
	} else {
		cfg.LessDisk = false
	}

	man.SnapshootNumber = ctx.GlobalUint64(SynSnapshootNumFlg.Name)
	man.SnapshootHash = ctx.GlobalString(SynSnapshootHashFlg.Name)
	man.SaveSnapStart = ctx.GlobalUint64(SaveSnapStartFlg.Name)
	man.SaveSnapPeriod = ctx.GlobalUint64(SaveSnapPeriodFlg.Name)
	man.SnaploadFromLocal = ctx.GlobalInt(SnapModeFlg.Name)
}

func setGPO(ctx *cli.Context, cfg *gasprice.Config) {
	if ctx.GlobalIsSet(GpoBlocksFlag.Name) {
		cfg.Blocks = ctx.GlobalInt(GpoBlocksFlag.Name)
	}
	if ctx.GlobalIsSet(GpoPercentileFlag.Name) {
		cfg.Percentile = ctx.GlobalInt(GpoPercentileFlag.Name)
	}
}

func setTxPool(ctx *cli.Context, cfg *core.TxPoolConfig) {
	//if ctx.GlobalIsSet(TxPoolNoLocalsFlag.Name) { //Y
	//	cfg.NoLocals = ctx.GlobalBool(TxPoolNoLocalsFlag.Name)
	//}
	//if ctx.GlobalIsSet(TxPoolJournalFlag.Name) {
	//	cfg.Journal = ctx.GlobalString(TxPoolJournalFlag.Name)
	//}
	//if ctx.GlobalIsSet(TxPoolRejournalFlag.Name) {
	//	cfg.Rejournal = ctx.GlobalDuration(TxPoolRejournalFlag.Name)
	//}
	if ctx.GlobalIsSet(TxPoolPriceLimitFlag.Name) {
		cfg.PriceLimit = ctx.GlobalUint64(TxPoolPriceLimitFlag.Name)
	}
	//if ctx.GlobalIsSet(TxPoolPriceBumpFlag.Name) {//Y
	//	cfg.PriceBump = ctx.GlobalUint64(TxPoolPriceBumpFlag.Name)
	//}
	if ctx.GlobalIsSet(TxPoolAccountSlotsFlag.Name) {
		cfg.AccountSlots = ctx.GlobalUint64(TxPoolAccountSlotsFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolGlobalSlotsFlag.Name) {
		cfg.GlobalSlots = ctx.GlobalUint64(TxPoolGlobalSlotsFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolAccountQueueFlag.Name) {
		cfg.AccountQueue = ctx.GlobalUint64(TxPoolAccountQueueFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolGlobalQueueFlag.Name) {
		cfg.GlobalQueue = ctx.GlobalUint64(TxPoolGlobalQueueFlag.Name)
	}
	//if ctx.GlobalIsSet(TxPoolLifetimeFlag.Name) {//Y
	//	cfg.Lifetime = ctx.GlobalDuration(TxPoolLifetimeFlag.Name)
	//}
}

func setManash(ctx *cli.Context, cfg *man.Config) {
	if ctx.GlobalIsSet(ManashCacheDirFlag.Name) {
		cfg.Manash.CacheDir = ctx.GlobalString(ManashCacheDirFlag.Name)
	}
	if ctx.GlobalIsSet(ManashDatasetDirFlag.Name) {
		cfg.Manash.DatasetDir = ctx.GlobalString(ManashDatasetDirFlag.Name)
	}
	if ctx.GlobalIsSet(ManashCachesInMemoryFlag.Name) {
		cfg.Manash.CachesInMem = ctx.GlobalInt(ManashCachesInMemoryFlag.Name)
	}
	if ctx.GlobalIsSet(ManashCachesOnDiskFlag.Name) {
		cfg.Manash.CachesOnDisk = ctx.GlobalInt(ManashCachesOnDiskFlag.Name)
	}
	if ctx.GlobalIsSet(ManashDatasetsInMemoryFlag.Name) {
		cfg.Manash.DatasetsInMem = ctx.GlobalInt(ManashDatasetsInMemoryFlag.Name)
	}
	if ctx.GlobalIsSet(ManashDatasetsOnDiskFlag.Name) {
		cfg.Manash.DatasetsOnDisk = ctx.GlobalInt(ManashDatasetsOnDiskFlag.Name)
	}
}

// checkExclusive verifies that only a single isntance of the provided flags was
// set by the user. Each flag might optionally be followed by a string type to
// specialize it further.
func checkExclusive(ctx *cli.Context, args ...interface{}) {
	set := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		// Make sure the next argument is a flag and skip if not set
		flag, ok := args[i].(cli.Flag)
		if !ok {
			panic(fmt.Sprintf("invalid argument, not cli.Flag type: %T", args[i]))
		}
		// Check if next arg extends current and expand its name if so
		name := flag.GetName()

		if i+1 < len(args) {
			switch option := args[i+1].(type) {
			case string:
				// Extended flag, expand the name and shift the arguments
				if ctx.GlobalString(flag.GetName()) == option {
					name += "=" + option
				}
				i++

			case cli.Flag:
			default:
				panic(fmt.Sprintf("invalid argument, not cli.Flag or string extension: %T", args[i+1]))
			}
		}
		// Mark the flag if it's set
		if ctx.GlobalIsSet(flag.GetName()) {
			set = append(set, "--"+name)
		}
	}
	if len(set) > 1 {
		Fatalf("Flags %v can't be used at the same time", strings.Join(set, ", "))
	}
}

// SetManConfig applies man-related command line flags to the config.
func SetManConfig(ctx *cli.Context, stack *pod.Node, cfg *man.Config) {
	// Avoid conflicting network flags
	checkExclusive(ctx, DeveloperFlag, TestnetFlag, RinkebyFlag)
	checkExclusive(ctx, FastSyncFlag, LightModeFlag, SyncModeFlag)
	checkExclusive(ctx, LightServFlag, LightModeFlag)
	checkExclusive(ctx, LightServFlag, SyncModeFlag, "light")

	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	setManerbase(ctx, ks, cfg)
	setGPO(ctx, &cfg.GPO)
	setTxPool(ctx, &cfg.TxPool)
	setManash(ctx, cfg)

	switch {
	case ctx.GlobalIsSet(SyncModeFlag.Name):
		cfg.SyncMode = *GlobalTextMarshaler(ctx, SyncModeFlag.Name).(*downloader.SyncMode)
	case ctx.GlobalBool(FastSyncFlag.Name):
		cfg.SyncMode = downloader.FastSync
	case ctx.GlobalBool(LightModeFlag.Name):
		cfg.SyncMode = downloader.LightSync
	}
	if ctx.GlobalIsSet(LightServFlag.Name) {
		cfg.LightServ = ctx.GlobalInt(LightServFlag.Name)
	}
	if ctx.GlobalIsSet(LightPeersFlag.Name) {
		cfg.LightPeers = ctx.GlobalInt(LightPeersFlag.Name)
	}
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.NetworkId = ctx.GlobalUint64(NetworkIdFlag.Name)
	}

	if ctx.GlobalIsSet(CacheFlag.Name) || ctx.GlobalIsSet(CacheDatabaseFlag.Name) {
		cfg.DatabaseCache = ctx.GlobalInt(CacheFlag.Name) * ctx.GlobalInt(CacheDatabaseFlag.Name) / 100
	}
	//if ctx.GlobalIsSet(DbTableSizeFlag.Name) {
	cfg.DatabaseTableSize = ctx.GlobalInt(DbTableSizeFlag.Name)
	//}
	cfg.DatabaseHandles = makeDatabaseHandles()

	if gcmode := ctx.GlobalString(GCModeFlag.Name); gcmode != "full" && gcmode != "archive" {
		Fatalf("--%s must be either 'full' or 'archive'", GCModeFlag.Name)
	}
	cfg.NoPruning = ctx.GlobalString(GCModeFlag.Name) == "archive"

	if ctx.GlobalIsSet(CacheFlag.Name) || ctx.GlobalIsSet(CacheGCFlag.Name) {
		cfg.TrieCache = ctx.GlobalInt(CacheFlag.Name) * ctx.GlobalInt(CacheGCFlag.Name) / 100
	}
	if ctx.GlobalIsSet(MinerThreadsFlag.Name) {
		cfg.MinerThreads = ctx.GlobalInt(MinerThreadsFlag.Name)
	}
	if ctx.GlobalIsSet(DocRootFlag.Name) {
		cfg.DocRoot = ctx.GlobalString(DocRootFlag.Name)
	}
	if ctx.GlobalIsSet(ExtraDataFlag.Name) {
		cfg.ExtraData = []byte(ctx.GlobalString(ExtraDataFlag.Name))
	}
	if ctx.GlobalIsSet(GasPriceFlag.Name) {
		cfg.GasPrice = GlobalBig(ctx, GasPriceFlag.Name)
	}
	if ctx.GlobalIsSet(VMEnableDebugFlag.Name) {
		// TODO(fjl): force-enable this in --dev mode
		cfg.EnablePreimageRecording = ctx.GlobalBool(VMEnableDebugFlag.Name)
	}

	// Override any default configs for hard coded networks.
	/*switch {
	case ctx.GlobalBool(TestnetFlag.Name):
		if !ctx.GlobalIsSet(NetworkIdFlag.Name) {
			cfg.NetworkId = 3
		}
		cfg.Genesis = core.DefaultTestnetGenesisBlock()
	case ctx.GlobalBool(RinkebyFlag.Name):
		if !ctx.GlobalIsSet(NetworkIdFlag.Name) {
			cfg.NetworkId = 4
		}
		cfg.Genesis = core.DefaultRinkebyGenesisBlock()
	case ctx.GlobalBool(DeveloperFlag.Name):
		// Create new developer account or reuse existing one
		var (
			developer accounts.Account
			err       error
		)
		if accs := ks.Accounts(); len(accs) > 0 {
			developer = ks.Accounts()[0]
		} else {
			developer, err = ks.NewAccount("")
			if err != nil {
				Fatalf("Failed to create developer account: %v", err)
			}
		}
		if err := ks.Unlock(developer, ""); err != nil {
			Fatalf("Failed to unlock developer account: %v", err)
		}
		log.Info("Using developer account", "address", developer.Address)

		cfg.Genesis = core.DeveloperGenesisBlock(uint64(ctx.GlobalInt(DeveloperPeriodFlag.Name)), developer.Address)
		if !ctx.GlobalIsSet(GasPriceFlag.Name) {
			cfg.GasPrice = big.NewInt(1)
		}
	}*/
	// TODO(fjl): move trie cache generations into config
	if gen := ctx.GlobalInt(TrieCacheGenFlag.Name); gen > 0 {
		state.MaxTrieCacheGen = uint16(gen)
	}
}

// SetDashboardConfig applies dashboard related command line flags to the config.
func SetDashboardConfig(ctx *cli.Context, cfg *dashboard.Config) {
	cfg.Host = ctx.GlobalString(DashboardAddrFlag.Name)
	cfg.Port = ctx.GlobalInt(DashboardPortFlag.Name)
	cfg.Refresh = ctx.GlobalDuration(DashboardRefreshFlag.Name)
}

// RegisterManService adds an Matrix client to the stack.
func RegisterManService(stack *pod.Node, cfg *man.Config) {
	var err error
	err = stack.Register(func(ctx *pod.ServiceContext) (pod.Service, error) {
		fullNode, err := man.New(ctx, cfg)
		return fullNode, err
	})
	if err != nil {
		Fatalf("Failed to register the Matrix service: %v", err)
	}
}

// RegisterDashboardService adds a dashboard to the stack.
func RegisterDashboardService(stack *pod.Node, cfg *dashboard.Config, commit string) {
	stack.Register(func(ctx *pod.ServiceContext) (pod.Service, error) {
		return dashboard.New(cfg, commit)
	})
}

// RegisterManStatsService configures the Matrix Stats daemon and adds it to
// th egiven node.
func RegisterManStatsService(stack *pod.Node, url string) {
	if err := stack.Register(func(ctx *pod.ServiceContext) (pod.Service, error) {
		// Retrieve both man and les services
		var manServ *man.Matrix
		ctx.Service(&manServ)

		return manstats.New(url, manServ)
	}); err != nil {
		Fatalf("Failed to register the Matrix Stats service: %v", err)
	}
}

// SetupNetwork configures the system for either the main net or some test network.
func SetupNetwork(ctx *cli.Context) {
	// TODO(fjl): move target gas limit into config
	params.TargetGasLimit = ctx.GlobalUint64(TargetGasLimitFlag.Name)
}

// MakeChainDatabase open an LevelDB using the flags passed to the client and will hard crash if it fails.
func MakeChainDatabase(ctx *cli.Context, stack *pod.Node) mandb.Database {
	var (
		cache   = ctx.GlobalInt(CacheFlag.Name) * ctx.GlobalInt(CacheDatabaseFlag.Name) / 100
		handles = makeDatabaseHandles()
	)
	name := "chaindata"
	if ctx.GlobalBool(LightModeFlag.Name) {
		name = "lightchaindata"
	}
	chainDb, err := stack.OpenDatabase(name, cache, handles)
	if err != nil {
		Fatalf("Could not open database: %v", err)
	}
	return chainDb
}

func MakeGenesis(ctx *cli.Context) *core.Genesis {
	var genesis *core.Genesis
	switch {
	case ctx.GlobalBool(TestnetFlag.Name):
		genesis = core.DefaultTestnetGenesisBlock()
	case ctx.GlobalBool(RinkebyFlag.Name):
		genesis = core.DefaultRinkebyGenesisBlock()
	case ctx.GlobalBool(DeveloperFlag.Name):
		Fatalf("Developer chains are ephemeral")
	}
	return genesis
}

// MakeChain creates a chain manager from set command line flags.
func MakeChain(ctx *cli.Context, stack *pod.Node) (chain *core.BlockChain, chainDb mandb.Database) {
	var err error
	chainDb = MakeChainDatabase(ctx, stack)

	config, _, err := core.SetupGenesisBlock(chainDb, MakeGenesis(ctx))
	if err != nil {
		Fatalf("%v", err)
	}

	engine, dposEngine := createEngineMap(ctx, stack, config, chainDb)

	if gcmode := ctx.GlobalString(GCModeFlag.Name); gcmode != "full" && gcmode != "archive" {
		Fatalf("--%s must be either 'full' or 'archive'", GCModeFlag.Name)
	}
	cache := &core.CacheConfig{
		Disabled:      ctx.GlobalString(GCModeFlag.Name) == "archive",
		TrieNodeLimit: man.DefaultConfig.TrieCache,
		TrieTimeLimit: man.DefaultConfig.TrieTimeout,
	}
	if ctx.GlobalIsSet(CacheFlag.Name) || ctx.GlobalIsSet(CacheGCFlag.Name) {
		cache.TrieNodeLimit = ctx.GlobalInt(CacheFlag.Name) * ctx.GlobalInt(CacheGCFlag.Name) / 100
	}
	vmcfg := vm.Config{EnablePreimageRecording: ctx.GlobalBool(VMEnableDebugFlag.Name)}

	chain, err = core.NewBlockChain(chainDb, cache, config, vmcfg, engine, dposEngine)
	if err != nil {
		Fatalf("Can't create BlockChain: %v", err)
	}
	return chain, chainDb
}

func createEngineMap(ctx *cli.Context, stack *pod.Node, config *params.ChainConfig, chainDb mandb.Database) (map[string]consensus.Engine, map[string]consensus.DPOSEngine) {
	engineMap := make(map[string]consensus.Engine)
	var alphaEngine consensus.Engine
	if config.Clique != nil {
		alphaEngine = clique.New(config.Clique, chainDb)
	} else {
		alphaEngine = manash.NewFaker()
		if !ctx.GlobalBool(FakePoWFlag.Name) {
			alphaEngine = manash.New(manash.Config{
				CacheDir:       stack.ResolvePath(man.DefaultConfig.Manash.CacheDir),
				CachesInMem:    man.DefaultConfig.Manash.CachesInMem,
				CachesOnDisk:   man.DefaultConfig.Manash.CachesOnDisk,
				DatasetDir:     stack.ResolvePath(man.DefaultConfig.Manash.DatasetDir),
				DatasetsInMem:  man.DefaultConfig.Manash.DatasetsInMem,
				DatasetsOnDisk: man.DefaultConfig.Manash.DatasetsOnDisk,
			})
		}
	}
	aiMineEngine := amhash.New(amhash.Config{PowMode: amhash.ModeNormal, PictureStorePath: stack.ResolvePath("picstore")})
	aiMineEngine.SetThreads(-1) // Disable CPU mining

	engineMap[manversion.VersionAlpha] = alphaEngine
	engineMap[manversion.VersionBeta] = alphaEngine
	engineMap[manversion.VersionGamma] = alphaEngine
	engineMap[manversion.VersionDelta] = alphaEngine
	engineMap[manversion.VersionAIMine] = aiMineEngine

	dposEngineMap := make(map[string]consensus.DPOSEngine)
	alphaDposEngine := mtxdpos.NewMtxDPOS(config.SimpleMode)
	dposEngineMap[manversion.VersionAlpha] = alphaDposEngine
	dposEngineMap[manversion.VersionBeta] = alphaDposEngine
	dposEngineMap[manversion.VersionGamma] = alphaDposEngine
	dposEngineMap[manversion.VersionDelta] = alphaDposEngine
	dposEngineMap[manversion.VersionAIMine] = alphaDposEngine

	return engineMap, dposEngineMap
}

// MakeConsolePreloads retrieves the absolute paths for the console JavaScript
// scripts to preload before starting.
func MakeConsolePreloads(ctx *cli.Context) []string {
	// Skip preloading if there's nothing to preload
	if ctx.GlobalString(PreloadJSFlag.Name) == "" {
		return nil
	}
	// Otherwise resolve absolute paths and return them
	preloads := []string{}

	assets := ctx.GlobalString(JSpathFlag.Name)
	for _, file := range strings.Split(ctx.GlobalString(PreloadJSFlag.Name), ",") {
		preloads = append(preloads, common.AbsolutePath(assets, strings.TrimSpace(file)))
	}
	return preloads
}

// MigrateFlags sets the global flag from a local flag when it's set.
// This is a temporary function used for migrating old command/flags to the
// new format.
//
// e.g. gman account new --keystore /tmp/mykeystore --lightkdf
//
// is equivalent after calling this method with:
//
// gman --keystore /tmp/mykeystore --lightkdf account new
//
// This allows the use of the existing configuration functionality.
// When all flags are migrated this function can be removed and the existing
// configuration functionality must be changed that is uses local flags
func MigrateFlags(action func(ctx *cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		for _, name := range ctx.FlagNames() {
			if ctx.IsSet(name) {
				ctx.GlobalSet(name, ctx.String(name))
			}
		}
		return action(ctx)
	}
}
