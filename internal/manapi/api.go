// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package manapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/MatrixAINetwork/go-matrix/params/manversion"

	"github.com/MatrixAINetwork/go-matrix/depoistInfo"

	"github.com/davecgh/go-spew/spew"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/MatrixAINetwork/go-matrix/accounts"
	"github.com/MatrixAINetwork/go-matrix/accounts/keystore"
	"github.com/MatrixAINetwork/go-matrix/base58"
	"github.com/MatrixAINetwork/go-matrix/ca"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/common/hexutil"
	"github.com/MatrixAINetwork/go-matrix/common/math"
	"github.com/MatrixAINetwork/go-matrix/consensus/manash"
	"github.com/MatrixAINetwork/go-matrix/console"
	"github.com/MatrixAINetwork/go-matrix/core"
	"github.com/MatrixAINetwork/go-matrix/core/matrixstate"
	"github.com/MatrixAINetwork/go-matrix/core/rawdb"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/core/vm"
	"github.com/MatrixAINetwork/go-matrix/core/vm/validatorGroup"
	"github.com/MatrixAINetwork/go-matrix/crc8"
	"github.com/MatrixAINetwork/go-matrix/crypto"
	"github.com/MatrixAINetwork/go-matrix/crypto/aes"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/mc"
	"github.com/MatrixAINetwork/go-matrix/p2p"
	"github.com/MatrixAINetwork/go-matrix/params"
	"github.com/MatrixAINetwork/go-matrix/params/enstrust"
	"github.com/MatrixAINetwork/go-matrix/rlp"
	"github.com/MatrixAINetwork/go-matrix/rpc"
)

const (
	defaultGasPrice = 50 * params.Shannon
)

// PublicMatrixAPI provides an API to access Matrix related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicMatrixAPI struct {
	b Backend
}

// NewPublicMatrixAPI creates a new Matrix protocol API.
func NewPublicMatrixAPI(b Backend) *PublicMatrixAPI {
	return &PublicMatrixAPI{b}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicMatrixAPI) GasPrice(ctx context.Context) (*big.Int, error) {
	//return s.b.SuggestPrice(ctx)
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return nil, err
	}
	gasprice, err := matrixstate.GetTxpoolGasLimit(state)
	if err != nil {
		return nil, err
	}
	return gasprice, nil
}

// ProtocolVersion returns the current Matrix protocol version this node supports
func (s *PublicMatrixAPI) ProtocolVersion() hexutil.Uint {
	return hexutil.Uint(s.b.ProtocolVersion())
}

// Syncing returns false in case the node is currently not syncing with the network. It can be up to date or has not
// yet received the latest block headers from its pears. In case it is synchronizing:
// - startingBlock: block number this node started to synchronise from
// - currentBlock:  block number this node is currently importing
// - highestBlock:  block number of the highest block header this node has received from peers
// - pulledStates:  number of state entries processed until now
// - knownStates:   number of known state entries that still need to be pulled
func (s *PublicMatrixAPI) Syncing() (interface{}, error) {
	progress := s.b.Downloader().Progress()

	// Return not syncing if the synchronisation already completed
	if progress.CurrentBlock >= progress.HighestBlock {
		return false, nil
	}
	// Otherwise gather the block sync stats
	return map[string]interface{}{
		"startingBlock": hexutil.Uint64(progress.StartingBlock),
		"currentBlock":  hexutil.Uint64(progress.CurrentBlock),
		"highestBlock":  hexutil.Uint64(progress.HighestBlock),
		"pulledStates":  hexutil.Uint64(progress.PulledStates),
		"knownStates":   hexutil.Uint64(progress.KnownStates),
	}, nil
}

// PublicTxPoolAPI offers and API for the transaction pool. It only operates on data that is non confidential.
type PublicTxPoolAPI struct {
	b Backend
}

// NewPublicTxPoolAPI creates a new tx pool service that gives information about the transaction pool.
func NewPublicTxPoolAPI(b Backend) *PublicTxPoolAPI {
	return &PublicTxPoolAPI{b}
}

// Content returns the transactions contained within the transaction pool.
func (s *PublicTxPoolAPI) Content() map[string]map[string]map[string]*RPCTransaction {
	content := map[string]map[string]map[string]*RPCTransaction{
		"pending": make(map[string]map[string]*RPCTransaction),
		"queued":  make(map[string]map[string]*RPCTransaction),
	}
	pending, queue := s.b.TxPoolContent()

	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

// Status returns the number of pending and queued transaction in the pool.
func (s *PublicTxPoolAPI) Status() map[string]hexutil.Uint {
	pending, queue := s.b.Stats()
	return map[string]hexutil.Uint{
		"pending": hexutil.Uint(pending),
		"queued":  hexutil.Uint(queue),
	}
}
func (s *PublicTxPoolAPI) GetTxNmap() map[uint32]common.Hash {
	nmap := s.b.GetTxNmap()
	retval := make(map[uint32]common.Hash)
	for k, v := range nmap {
		retval[k] = v.Hash()
	}
	return retval
}

// Inspect retrieves the content of the transaction pool and flattens it into an
// easily inspectable list.
func (s *PublicTxPoolAPI) Inspect() map[string]map[string]map[string]string {
	content := map[string]map[string]map[string]string{
		"pending": make(map[string]map[string]string),
		"queued":  make(map[string]map[string]string),
	}
	pending, queue := s.b.TxPoolContent()

	// Define a formatter to flatten a transaction into a string
	var format = func(tx types.SelfTransaction) string {
		if to := tx.To(); to != nil {
			return fmt.Sprintf("%s: %v wei + %v gas × %v wei", tx.To().Hex(), tx.Value(), tx.Gas(), tx.GasPrice())
		}
		return fmt.Sprintf("contract creation: %v wei + %v gas × %v wei", tx.Value(), tx.Gas(), tx.GasPrice())
	}
	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

// PublicAccountAPI provides an API to access accounts managed by this node.
// It offers only methods that can retrieve accounts.
type PublicAccountAPI struct {
	b Backend
	//am *accounts.Manager
}

// NewPublicAccountAPI creates a new PublicAccountAPI.
func NewPublicAccountAPI(b Backend /*,am *accounts.Manager*/) *PublicAccountAPI {
	return &PublicAccountAPI{b: b}
}

// Accounts returns the collection of accounts this node manages
func (s *PublicAccountAPI) Accounts() [][]string {
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return nil
	}
	coinlist, err := core.GetMatrixCoin(state)
	if err != nil {
		return nil
	}
	var tmpstr string
	var strMulAddrList [][]string
	for _, wallet := range s.b.AccountManager().Wallets() {
		for _, account := range wallet.Accounts() {
			var mulAccounts [][]string
			accountlist := make([]string, 0)
			strAddr := base58.Base58EncodeToString(params.MAN_COIN, account.Address)
			if tmpstr == strAddr {
				continue
			}
			tmpstr = strAddr
			accountlist = append(accountlist, tmpstr)
			for _, coin := range coinlist {
				accountlist = append(accountlist, base58.Base58EncodeToString(coin, account.Address))
			}
			mulAccounts = append(mulAccounts, accountlist)
			strMulAddrList = append(strMulAddrList, mulAccounts...)
		}
	}

	return strMulAddrList
}

// PrivateAccountAPI provides an API to access accounts managed by this node.
// It offers methods to create, (un)lock en list accounts. Some methods accept
// passwords and are therefore considered private by default.
type PrivateAccountAPI struct {
	am        *accounts.Manager
	nonceLock *AddrLocker
	b         Backend
}

// NewPrivateAccountAPI create a new PrivateAccountAPI.
func NewPrivateAccountAPI(b Backend, nonceLock *AddrLocker) *PrivateAccountAPI {
	return &PrivateAccountAPI{
		am:        b.AccountManager(),
		nonceLock: nonceLock,
		b:         b,
	}
}

// ListAccounts will return a list of addresses for accounts this node manages.
func (s *PrivateAccountAPI) ListAccounts() []common.Address {
	addresses := make([]common.Address, 0) // return [] instead of nil if empty
	for _, wallet := range s.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address)
		}
	}
	return addresses
}

// rawWallet is a JSON representation of an accounts.Wallet interface, with its
// data contents extracted into plain fields.
type rawWallet struct {
	URL      string             `json:"url"`
	Status   string             `json:"status"`
	Failure  string             `json:"failure,omitempty"`
	Accounts []accounts.Account `json:"accounts,omitempty"`
}

// ListWallets will return a list of wallets this node manages.
func (s *PrivateAccountAPI) ListWallets() []rawWallet {
	wallets := make([]rawWallet, 0) // return [] instead of nil if empty
	for _, wallet := range s.am.Wallets() {
		status, failure := wallet.Status()

		raw := rawWallet{
			URL:      wallet.URL().String(),
			Status:   status,
			Accounts: wallet.Accounts(),
		}
		if failure != nil {
			raw.Failure = failure.Error()
		}
		wallets = append(wallets, raw)
	}
	return wallets
}

// OpenWallet initiates a hardware wallet opening procedure, establishing a USB
// connection and attempting to authenticate via the provided passphrase. Note,
// the method may return an extra challenge requiring a second open (e.g. the
// Trezor PIN matrix challenge).
func (s *PrivateAccountAPI) OpenWallet(url string, passphrase *string) error {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return err
	}
	pass := ""
	if passphrase != nil {
		pass = *passphrase
	}
	return wallet.Open(pass)
}

// DeriveAccount requests a HD wallet to derive a new account, optionally pinning
// it for later reuse.
func (s *PrivateAccountAPI) DeriveAccount(url string, path string, pin *bool) (accounts.Account, error) {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return accounts.Account{}, err
	}
	derivPath, err := accounts.ParseDerivationPath(path)
	if err != nil {
		return accounts.Account{}, err
	}
	if pin == nil {
		pin = new(bool)
	}
	return wallet.Derive(derivPath, *pin)
}

// NewAccount will create a new account and returns the address for the new account.
//func (s *PrivateAccountAPI) NewAccount(password string) (common.Address, error) {
//	acc, err := fetchKeystore(s.am).NewAccount(password)
//	if err == nil {
//		return acc.Address, nil
//	}
//	return common.Address{}, err
//}
func (s *PrivateAccountAPI) NewAccount(password string) (string, error) {
	acc, err := fetchKeystore(s.am).NewAccount(password)
	if err == nil {
		return acc.ManAddress(), nil
	}
	return "", err
}

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

// ImportRawKey stores the given hex encoded ECDSA key into the key directory,
// encrypting it with the passphrase.
func (s *PrivateAccountAPI) ImportRawKey(privkey string, password string) (string, error) {
	key, err := crypto.HexToECDSA(privkey)
	if err != nil {
		return "", err
	}
	acc, err := fetchKeystore(s.am).ImportECDSA(key, password)
	return acc.ManAddress(), err
}
func GetPassword() (string, error) {
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		return "", fmt.Errorf("Failed to read passphrase: %v", err)
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

func (s *PrivateAccountAPI) SetEntrustSignAccount(path string, password string) (string, error) {

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(f)
	bytesPass, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return "", errors.New("the contents of the file '" + path + "' are incorrect")
	}
	h := sha256.New()
	h.Write([]byte(password))
	tpass, err := aes.AesDecrypt(bytesPass, h.Sum(nil))
	if err != nil {
		return "", err
	}

	var anss []mc.EntrustInfo
	err = json.Unmarshal(tpass, &anss)
	if err != nil {
		return "", errors.New("incorrect password")
	}
	entrustValue := make(map[common.Address]string, 0)

	for _, v := range anss {
		addr, err := base58.Base58DecodeToAddress(v.Address)
		if err != nil {
			return "", err
		}
		entrustValue[addr] = v.Password
	}
	err = entrust.EntrustAccountValue.SetEntrustValue(entrustValue)
	if err != nil {
		return "", err
	}
	return "successful", nil
}

// UnlockAccount will unlock the account associated with the given address with
// the given password for duration seconds. If duration is nil it will use a
// default of 300 seconds. It returns an indication if the account was unlocked.
func (s *PrivateAccountAPI) UnlockAccount(strAddr string, password string, duration *uint64) (bool, error) {
	const max = uint64(time.Duration(math.MaxInt64) / time.Second)
	var d time.Duration
	if duration == nil {
		d = 300 * time.Second
	} else if *duration > max {
		return false, errors.New("unlock duration too large")
	} else {
		d = time.Duration(*duration) * time.Second
	}
	addr, err := base58.Base58DecodeToAddress(strAddr)
	if err != nil {
		return false, err
	}
	err = fetchKeystore(s.am).TimedUnlock(accounts.Account{Address: addr}, password, d)
	return err == nil, err
}

// LockAccount will lock the account associated with the given address when it's unlocked.
func (s *PrivateAccountAPI) LockAccount(strAddr string) bool {
	addr, err := base58.Base58DecodeToAddress(strAddr)
	if err != nil {
		return false
	}
	return fetchKeystore(s.am).Lock(addr) == nil
}

// signTransactions sets defaults and signs the given transaction
// NOTE: the caller needs to ensure that the nonceLock is held, if applicable,
// and release it after the transaction has been submitted to the tx pool
//var txcountaaaaaaa = uint64(0)
func (s *PrivateAccountAPI) signTransaction(ctx context.Context, args SendTxArgs, passwd string) (types.SelfTransaction, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}
	wallet, err := s.am.Find(account)
	if err != nil {
		return nil, err
	}
	// Set some sanity defaults and terminate on failure
	if err := args.setDefaults(ctx, s.b); err != nil {
		return nil, err
	}

	// Assemble the transaction and sign with the wallet
	tx := args.toTransaction()

	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	return wallet.SignTxWithPassphrase(account, passwd, tx, chainID)
}

// SendTransaction will create a transaction from the given arguments and
// tries to sign it with the key associated with args.To. If the given passwd isn't
// able to decrypt the key it fails.
func (s *PrivateAccountAPI) SendTransaction(ctx context.Context, args1 SendTxArgs1, passwd string) (common.Hash, error) {
	if args1.TxType == common.ExtraBroadTxType {
		return common.Hash{}, errors.New("TxType can not be set 1")
	}
	var args SendTxArgs
	args, err := StrArgsToByteArgs(args1)
	if err != nil {
		return common.Hash{}, err
	}
	if args.Nonce == nil {
		// Hold the addresse's mutex around signing to prevent concurrent assignment of
		// the same nonce to multiple accounts.
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	signed, err := s.signTransaction(ctx, args, passwd)
	if err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, signed)
}

// SignTransaction will create a transaction from the given arguments and
// tries to sign it with the key associated with args.To. If the given passwd isn't
// able to decrypt the key it fails. The transaction is returned in RLP-form, not broadcast
// to other nodes
func (s *PrivateAccountAPI) SignTransaction(ctx context.Context, args SendTxArgs, passwd string) (*SignTransactionResult, error) {
	// No need to obtain the noncelock mutex, since we won't be sending this
	// tx into the transaction pool, but right back to the user
	if args.Gas == nil {
		return nil, fmt.Errorf("gas not specified")
	}
	if args.GasPrice == nil {
		return nil, fmt.Errorf("gasPrice not specified")
	}
	if args.Nonce == nil {
		return nil, fmt.Errorf("nonce not specified")
	}
	signed, err := s.signTransaction(ctx, args, passwd)
	if err != nil {
		return nil, err
	}
	data, err := rlp.EncodeToBytes(signed)
	if err != nil {
		return nil, err
	}
	return &SignTransactionResult{data, signed}, nil
}

// signHash is a helper function that calculates a hash for the given message that can be
// safely used to calculate a signature from.
//
// The hash is calulcated as
//   keccak256("\x19Matrix Signed Message:\n"${message length}${message}).
//
// This gives context to the signed message and prevents signing of transactions.
func signHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Matrix Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}

// Sign calculates an Matrix ECDSA signature for:
// keccack256("\x19Matrix Signed Message:\n" + len(message) + message))
//
// Note, the produced signature conforms to the secp256k1 curve R, S and V values,
// where the V value will be 27 or 28 for legacy reasons.
//
// The key used to calculate the signature is decrypted with the given password.
//
// https://github.com/MatrixAINetwork/go-matrix/wiki/Management-APIs#personal_sign
func (s *PrivateAccountAPI) Sign(ctx context.Context, data hexutil.Bytes, strAddr string, passwd string) (hexutil.Bytes, error) {
	// Look up the wallet containing the requested signer
	addr, err := base58.Base58DecodeToAddress(strAddr)
	if err != nil {
		return nil, err
	}
	account := accounts.Account{Address: addr}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	// Assemble sign the data with the wallet
	signature, err := wallet.SignHashWithPassphrase(account, passwd, signHash(data))
	if err != nil {
		return nil, err
	}
	signature[64] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	return signature, nil
}

// EcRecover returns the address for the account that was used to create the signature.
// Note, this function is compatible with man_sign and personal_sign. As such it recovers
// the address of:
// hash = keccak256("\x19Matrix Signed Message:\n"${message length}${message})
// addr = ecrecover(hash, signature)
//
// Note, the signature must conform to the secp256k1 curve R, S and V values, where
// the V value must be be 27 or 28 for legacy reasons.
//
// https://github.com/MatrixAINetwork/go-matrix/wiki/Management-APIs#personal_ecRecover
func (s *PrivateAccountAPI) EcRecover(ctx context.Context, data, sig hexutil.Bytes) (common.Address, error) {
	if len(sig) != 65 {
		return common.Address{}, fmt.Errorf("signature must be 65 bytes long")
	}
	if sig[64] != 27 && sig[64] != 28 {
		return common.Address{}, fmt.Errorf("invalid Matrix signature (V is not 27 or 28)")
	}
	sig[64] -= 27 // Transform yellow paper V from 27/28 to 0/1

	rpk, err := crypto.Ecrecover(signHash(data), sig)
	if err != nil {
		return common.Address{}, err
	}
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return recoveredAddr, nil
}

// SignAndSendTransaction was renamed to SendTransaction. This method is deprecated
// and will be removed in the future. It primary goal is to give clients time to update.
func (s *PrivateAccountAPI) SignAndSendTransaction(ctx context.Context, args SendTxArgs1, passwd string) (common.Hash, error) {
	return s.SendTransaction(ctx, args, passwd)
}

// PublicBlockChainAPI provides an API to access the Matrix blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b Backend
}

// NewPublicBlockChainAPI creates a new Matrix blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockChainAPI) BlockNumber() *big.Int {
	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	return header.Number
}

type RPCBalanceType struct {
	AccountType uint32       `json:"accountType"`
	Balance     *hexutil.Big `json:"balance"`
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, strAddress string, blockNr rpc.BlockNumber) ([]RPCBalanceType, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	var cointype string
	strlist := strings.Split(strAddress, ".")
	if len(strlist) > 1 {
		cointype = strlist[0]
	} else {
		return nil, errors.New("Illegal input address")
	}
	if cointype == "" {
		return nil, errors.New("Invalid currency")
	}
	address, err := base58.Base58DecodeToAddress(strAddress)
	if err != nil {
		return nil, err
	}
	var balance []RPCBalanceType
	b := state.GetBalance(cointype, address)
	if b == nil {
		tmp := new(RPCBalanceType)
		var i uint32
		for i = 0; i <= common.LastAccount; i++ {
			tmp.AccountType = i
			tmp.Balance = new(hexutil.Big)
			balance = append(balance, *tmp)
		}
	} else {
		for i := 0; i < len(b); i++ {
			balance = append(balance, RPCBalanceType{b[i].AccountType, (*hexutil.Big)(b[i].Balance)})
		}
	}
	return balance, state.Error()
}
func (s *PublicBlockChainAPI) GetMatrixCoin(ctx context.Context, blockNr rpc.BlockNumber) ([]string, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	bs := state.GetMatrixData(types.RlpHash(params.COIN_NAME))
	var tmpcoinlist []string
	if len(bs) > 0 {
		err := json.Unmarshal(bs, &tmpcoinlist)
		if err != nil {
			log.Trace("get matrix coin", "unmarshal err", err)
			return nil, err
		}
	}
	var coinlist []string
	for _, coin := range tmpcoinlist {
		if !common.IsValidityCurrency(coin) {
			continue
		}
		coinlist = append(coinlist, coin)
	}
	return coinlist, nil
}

func (s *PublicBlockChainAPI) GetDestroyBalance(ctx context.Context, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	bs := state.GetMatrixData(types.RlpHash(params.COIN_NAME))
	var tmpcoinlist []string
	if len(bs) > 0 {
		err := json.Unmarshal(bs, &tmpcoinlist)
		if err != nil {
			log.Trace("get matrix coin", "unmarshal err", err)
			return nil, err
		}
	}
	var coinlist []string
	for _, coin := range tmpcoinlist {
		if !common.IsValidityCurrency(coin) {
			continue
		}
		coinlist = append(coinlist, coin)
	}

	value, _ := new(big.Int).SetString(params.DestroyBalance, 0)
	for i := 0; i < len(coinlist)/params.CoinDampingNum; i++ {
		tmpa := big.NewInt(95)
		tmpb := big.NewInt(100)
		value.Mul(value, tmpa)
		value.Quo(value, tmpb)
	}
	return value, nil
}
func (s *PublicBlockChainAPI) GetMatrixCoinConfig(ctx context.Context, cointpy string, blockNr rpc.BlockNumber) ([]common.CoinConfig, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	bs := state.GetMatrixData(types.RlpHash(common.COINPREFIX + mc.MSCurrencyConfig))
	var tmpcoinlist []common.CoinConfig
	if len(bs) > 0 {
		//err := rlp.DecodeBytes(bs, &tmpcoinlist)
		err := json.Unmarshal(bs, &tmpcoinlist)
		if err != nil {
			log.Trace("get matrix coin", "unmarshal err", err)
			return nil, err
		}
	}
	var coinlist []common.CoinConfig
	for _, coin := range tmpcoinlist {
		if !common.IsValidityCurrency(coin.CoinType) {
			continue
		}
		if cointpy == "" {
			coinlist = append(coinlist, coin)
			continue
		}
		if cointpy == coin.CoinType {
			coinlist = append(coinlist, coin)
			break
		}
	}
	return coinlist, nil
}
func (s *PublicBlockChainAPI) GetUpTime(ctx context.Context, strAddress string, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	address, err := base58.Base58DecodeToAddress(strAddress)
	if err != nil {
		return nil, err
	}
	read, _ := depoistInfo.GetOnlineTime(state, address)
	return read, state.Error()
}
func (s *PublicBlockChainAPI) GetInterest(ctx context.Context, strAddress string, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	address, _ := base58.Base58DecodeToAddress(strAddress)

	read, _ := depoistInfo.GetInterest(state, address)

	return (*hexutil.Big)(read), state.Error()
}

func (s *PublicBlockChainAPI) GetSlash(ctx context.Context, strAddress string, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	address, _ := base58.Base58DecodeToAddress(strAddress)

	read, _ := depoistInfo.GetSlash(state, address)

	return (*hexutil.Big)(read), state.Error()
}

type DepositDetail struct {
	Address     string
	SignAddress string
	Deposit     *big.Int
	WithdrawH   *big.Int
	OnlineTime  *big.Int
	Role        *big.Int
}
type RpcValidatorGroupState struct {
	OwnerInfo    RpcOwnerInfo
	Reward       vm.RewardRate
	ValidatorMap []RpcValidatorInfo
}
type RpcOwnerInfo struct {
	Owner           string //common.Address
	WithdrawAllTime uint64
	SignAddress     string //common.Address `rlp:"-"`
}
type RpcValidatorInfo struct {
	Address   string //common.Address
	Reward    *big.Int
	AllAmount *big.Int //`rlp:"-"`
	Current   validatorGroup.CurrentData
	Positions []validatorGroup.DepositPos
}

func (s *PublicBlockChainAPI) GetValidatorGroupInfo(ctx context.Context, blockNr rpc.BlockNumber) (interface{}, error) {
	state, header, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || header == nil || err != nil {
		return nil, err
	}
	vcStates := &vm.ValidatorContractState{}
	getRetval, err := vcStates.GetValidatorGroupInfo(header.Time.Uint64(), state)
	if err != nil {
		return nil, err
	}
	info := MakeJsonInferface(&getRetval)
	return info, nil
}

//退选信息
type RpcWithDrawInfo struct {
	WithDrawAmount *hexutil.Big
	WithDrawTime   uint64 //退选时间
}

//没把定期DepositAmount和WithDrawTime放到ZeroDepositlist里，因为这样处理方便，暂时这样用
type RpcDepositMsg struct {
	DepositType      uint64 //0-活期,1-定期1个月,3-定期3个月,6-定期6个月
	DepositAmount    *hexutil.Big
	Interest         *hexutil.Big
	Slash            *hexutil.Big
	BeginTime        uint64 //定期起始时间，为当前确认时间(evm.Time)
	EndTime          uint64 //定期到期时间，(BeginTime+定期时长)
	Position         uint64 //仓位
	WithDrawInfolist []RpcWithDrawInfo
}
type RpcDepositBase struct {
	AddressA0     string
	AddressA1     string
	OnlineTime    *hexutil.Big
	Role          *hexutil.Big
	PositionNonce uint64
	Dpstmsg       []RpcDepositMsg
}

func (s *PublicBlockChainAPI) GetDeposit(ctx context.Context, blockNr rpc.BlockNumber) ([]DepositDetail, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}

	depositNodes, err := ca.GetElectedByHeight(new(big.Int).SetInt64(int64(blockNr)))
	if nil != err {
		return nil, err
	}
	if 0 == len(depositNodes) {
		return nil, err
	}
	depositNodesOutput := make([]DepositDetail, 0)
	for _, v := range depositNodes {
		tmp := DepositDetail{Address: base58.Base58EncodeToString(params.MAN_COIN, v.Address), SignAddress: base58.Base58EncodeToString(params.MAN_COIN, v.SignAddress), Deposit: v.Deposit, WithdrawH: v.WithdrawH, OnlineTime: v.OnlineTime, Role: v.Role}
		depositNodesOutput = append(depositNodesOutput, tmp)
	}
	return depositNodesOutput, state.Error()
}
func (s *PublicBlockChainAPI) GetDepositByAddr(ctx context.Context, straddr string, blockNr rpc.BlockNumber) (*RpcDepositBase, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	addr, err := base58.Base58DecodeToAddress(straddr)
	if err != nil {
		return nil, err
	}
	depositOutput := depoistInfo.GetDepositBase(state, addr)
	if depositOutput == nil {
		return nil, err
	}
	rpcbase := RpcDepositBase{
		AddressA1:     base58.Base58EncodeToString(params.MAN_COIN, depositOutput.AddressA1),
		AddressA0:     base58.Base58EncodeToString(params.MAN_COIN, depositOutput.AddressA0),
		OnlineTime:    new(hexutil.Big),
		Role:          new(hexutil.Big),
		PositionNonce: depositOutput.PositionNonce,
	}
	rpcbase.OnlineTime = (*hexutil.Big)(depositOutput.OnlineTime)
	rpcbase.Role = (*hexutil.Big)(depositOutput.Role)
	rpcbase.Dpstmsg = make([]RpcDepositMsg, 0)
	for _, deposit := range depositOutput.Dpstmsg {
		rpcmsg := RpcDepositMsg{
			DepositType:   deposit.DepositType,
			DepositAmount: new(hexutil.Big),
			Interest:      new(hexutil.Big),
			Slash:         new(hexutil.Big),
			BeginTime:     deposit.BeginTime,
			EndTime:       deposit.EndTime,
			Position:      deposit.Position,
		}
		rpcmsg.DepositAmount = (*hexutil.Big)(deposit.DepositAmount)
		rpcmsg.Interest = (*hexutil.Big)(deposit.Interest)
		rpcmsg.Slash = (*hexutil.Big)(deposit.Slash)
		rpcmsg.WithDrawInfolist = make([]RpcWithDrawInfo, 0)
		for _, wd := range deposit.WithDrawInfolist {
			rpcwd := RpcWithDrawInfo{
				WithDrawAmount: new(hexutil.Big),
				WithDrawTime:   wd.WithDrawTime,
			}
			rpcwd.WithDrawAmount = (*hexutil.Big)(wd.WithDrawAmount)
			rpcmsg.WithDrawInfolist = append(rpcmsg.WithDrawInfolist, rpcwd)
		}
		rpcbase.Dpstmsg = append(rpcbase.Dpstmsg, rpcmsg)
	}
	return &rpcbase, nil
}
func (api *PublicBlockChainAPI) GetFutureRewards(ctx context.Context, number rpc.BlockNumber) (interface{}, error) {
	state, _, err := api.b.StateAndHeaderByNumber(ctx, number-1)
	if state == nil || err != nil {
		return nil, err
	}
	return api.b.GetFutureRewards(state, number)
}

func getCoinFromManAddress(manAddress string) (string, error) {
	err := CheckParams(manAddress)
	if err != nil {
		return "", err
	}
	return strings.Split(manAddress, ".")[0], nil
}

//钱包调用
func (s *PublicBlockChainAPI) GetEntrustList(strAuthFrom string) []common.EntrustType {
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return nil
	}
	coin, err := getCoinFromManAddress(strAuthFrom)
	if err != nil {
		return nil
	}
	authFrom, err := base58.Base58DecodeToAddress(strAuthFrom)
	if err != nil {
		return nil
	}

	validEntrustList := make([]common.EntrustType, 0)
	allEntrustList := state.GetAllEntrustList(coin, authFrom)
	for _, entrustData := range allEntrustList {
		if entrustData.EnstrustSetType == params.EntrustByHeight {
			if s.b.CurrentBlock().NumberU64() <= entrustData.EndHeight {
				validEntrustList = append(validEntrustList, entrustData)
			}
		} else if entrustData.EnstrustSetType == params.EntrustByTime {
			if s.b.CurrentBlock().Time().Uint64() <= entrustData.EndTime {
				validEntrustList = append(validEntrustList, entrustData)
			}
		} else if entrustData.EnstrustSetType == params.EntrustByCount {
			if entrustData.EntrustCount > 0 {
				validEntrustList = append(validEntrustList, entrustData)
			}
		}
	}

	return validEntrustList
}

func (s *PublicBlockChainAPI) GetBlackList() []string {
	return common.BlackListString
}

func (s *PublicBlockChainAPI) GetIPFSfirstcache() {
	fmt.Println("ipfs get first cache list")
	s.b.Downloader().DGetIPFSfirstcache()
}
func (s *PublicBlockChainAPI) GetIPFSsecondcache(strhash string) {
	fmt.Println("ipfs get second cache list")
	s.b.Downloader().DGetIPFSSecondcache(strhash)
}

func (s *PublicBlockChainAPI) GetIPFSblock(strhash string) {
	fmt.Println("ipfs get block info")
	s.b.Downloader().DGetIPFSBlock(strhash)
}
func (s *PublicBlockChainAPI) GetIPFSsnap(str string) {
	fmt.Println("ipfs get snapshoot info") //getIPFScommon
	s.b.Downloader().DGetIPFSsnap(str)
}

func (s *PublicBlockChainAPI) GetAuthFrom(strEntrustFrom string, height uint64) string {
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return ""
	}
	coin, err := getCoinFromManAddress(strEntrustFrom)
	if err != nil {
		return ""
	}
	entrustFrom, err := base58.Base58DecodeToAddress(strEntrustFrom)
	if err != nil {
		return ""
	}
	addr := state.GetAuthFrom(coin, entrustFrom, height)
	if addr.Equal(common.Address{}) {
		return ""
	}
	return base58.Base58EncodeToString(coin, addr)
}
func (s *PublicBlockChainAPI) GetEntrustFrom(strAuthFrom string, height uint64) []string {
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return nil
	}
	coin, err := getCoinFromManAddress(strAuthFrom)
	if err != nil {
		return nil
	}
	entrustFrom, err := base58.Base58DecodeToAddress(strAuthFrom)
	if err != nil {
		return nil
	}
	addrList := state.GetEntrustFrom(coin, entrustFrom, height)
	var strAddrList []string
	for _, addr := range addrList {
		if !addr.Equal(common.Address{}) {
			strAddr := base58.Base58EncodeToString(coin, addr)
			strAddrList = append(strAddrList, strAddr)
		}
	}
	return strAddrList
}
func (s *PublicBlockChainAPI) GetAuthFromByTime(strEntrustFrom string, time uint64) string {
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return ""
	}
	coin, err := getCoinFromManAddress(strEntrustFrom)
	if err != nil {
		return ""
	}
	entrustFrom, err := base58.Base58DecodeToAddress(strEntrustFrom)
	if err != nil {
		return ""
	}
	addr := state.GetGasAuthFromByTime(coin, entrustFrom, time)
	if addr.Equal(common.Address{}) {
		return ""
	}
	return base58.Base58EncodeToString(coin, addr)
}
func (s *PublicBlockChainAPI) GetEntrustFromByTime(strAuthFrom string, time uint64) []string {
	state, err := s.b.GetState()
	if state == nil || err != nil {
		return nil
	}
	coin, err := getCoinFromManAddress(strAuthFrom)
	if err != nil {
		return nil
	}
	entrustFrom, err := base58.Base58DecodeToAddress(strAuthFrom)
	if err != nil {
		return nil
	}
	addrList := state.GetEntrustFromByTime(coin, entrustFrom, time)
	var strAddrList []string
	for _, addr := range addrList {
		if !addr.Equal(common.Address{}) {
			strAddr := base58.Base58EncodeToString(coin, addr)
			strAddrList = append(strAddrList, strAddr)
		}
	}
	return strAddrList
}

//钱包调用，根据块高查询委托gas授权地址
func (s *PublicBlockChainAPI) GetAuthGasAddress(ctx context.Context, strAddress string, blockNr rpc.BlockNumber) (string, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return "", err
	}
	coin, err := getCoinFromManAddress(strAddress)
	if err != nil {
		return "", err
	}
	addr, err := base58.Base58DecodeToAddress(strAddress)
	if err != nil {
		return "", err
	}
	authAddr := state.GetGasAuthFromByHeightAddTime(coin, addr)
	if !authAddr.Equal(common.Address{}) {
		return base58.Base58EncodeToString(coin, authAddr), nil
	}
	return "", errors.New("without entrust gas")
}

func (s *PublicBlockChainAPI) GetMatrixStateByNum(ctx context.Context, key string, blockNr rpc.BlockNumber) (interface{}, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}

	version := matrixstate.GetVersionInfo(state)
	if key == mc.MSKeyVersionInfo {
		return version, nil
	}
	mgr := matrixstate.GetManager(version)
	if mgr == nil {
		return nil, nil
	}
	//supMager := supertxsstate.GetManager(version)

	opt, err := mgr.FindOperator(key)
	if err != nil {
		log.Error("GetCfgDataByState:FindOperator failed", "key", key, "err", err)
		return nil, err
	}
	dataval, err := opt.GetValue(state)
	if err != nil {
		log.Error("GetCfgDataByState:SetValue failed", "err", err)
		return nil, err
	}
	//_, val := supMager.Output(key, dataval)

	return dataval, nil
}

func (s *PublicBlockChainAPI) GetGasPrice() *big.Int {
	return big.NewInt(int64(params.TxGasPrice))
}

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		//response, err := s.rpcOutputBlock(block, true, fullTx)
		response, err := s.rpcOutputBlock1(block, true, fullTx)
		if err == nil && blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		//return s.rpcOutputBlock(block, true, fullTx)
		return s.rpcOutputBlock1(block, true, fullTx)
	}
	return nil, err
}

// GetUncleByBlockNumberAndIndex returns the uncle block for the given block hash and index. When fullTx is true
// all transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetUncleByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		uncles := block.Uncles()
		if index >= hexutil.Uint(len(uncles)) {
			log.Debug("Requested uncle not found", "number", blockNr, "hash", block.Hash(), "index", index)
			return nil, nil
		}
		block = types.NewBlockWithHeader(uncles[index])
		return s.rpcOutputBlock(block, false, false)
	}
	return nil, err
}

// GetUncleByBlockHashAndIndex returns the uncle block for the given block hash and index. When fullTx is true
// all transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetUncleByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		uncles := block.Uncles()
		if index >= hexutil.Uint(len(uncles)) {
			log.Debug("Requested uncle not found", "number", block.Number(), "hash", blockHash, "index", index)
			return nil, nil
		}
		block = types.NewBlockWithHeader(uncles[index])
		return s.rpcOutputBlock(block, false, false)
	}
	return nil, err
}

// GetUncleCountByBlockNumber returns number of uncles in the block for the given block number
func (s *PublicBlockChainAPI) GetUncleCountByBlockNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		n := hexutil.Uint(len(block.Uncles()))
		return &n
	}
	return nil
}

// GetUncleCountByBlockHash returns number of uncles in the block for the given block hash
func (s *PublicBlockChainAPI) GetUncleCountByBlockHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		n := hexutil.Uint(len(block.Uncles()))
		return &n
	}
	return nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) getCode(ctx context.Context, address common.Address, cointype string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(cointype, address)
	return code, state.Error()
}
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, manAddress string, cointype string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	addres, err := base58.Base58DecodeToAddress(manAddress)
	if err != nil {

	}
	return s.getCode(ctx, addres, cointype, blockNr)
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) getStorageAt(ctx context.Context, address common.Address, key string, cointype string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(cointype, address, common.HexToHash(key))
	return res[:], state.Error()
}
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, manAddress string, key string, cointype string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	addres, err := base58.Base58DecodeToAddress(manAddress)
	if err != nil {
		return nil, err
	}
	return s.getStorageAt(ctx, addres, key, cointype, blockNr)
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Currency *string         `json:"currency"    gencodec:"required"`
	Gas      hexutil.Uint64  `json:"gas"`
	GasPrice hexutil.Big     `json:"gasPrice"`
	Value    hexutil.Big     `json:"value"`
	Data     hexutil.Bytes   `json:"data"`
	ExtraTo  []*ExtraTo_Mx   `json:"extra_to"` //
}
type ManCallArgs struct {
	From     string         `json:"from"`
	To       *string        `json:"to"`
	Currency *string        `json:"currency"`
	Gas      hexutil.Uint64 `json:"gas"`
	GasPrice hexutil.Big    `json:"gasPrice"`
	Value    hexutil.Big    `json:"value"`
	Data     hexutil.Bytes  `json:"data"`
	ExtraTo  []*ExtraTo_Mx1 `json:"extra_to"` //
}

func (s *PublicBlockChainAPI) doCall(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber, vmCfg vm.Config, timeout time.Duration) ([]byte, uint64, bool, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())

	state, header, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, 0, false, err
	}
	// Set sender address or use a default if none specified
	addr := args.From
	if addr == (common.Address{}) {
		if wallets := s.b.AccountManager().Wallets(); len(wallets) > 0 {
			if accounts := wallets[0].Accounts(); len(accounts) > 0 {
				addr = accounts[0].Address
			}
		}
	}
	// Set default gas & gas price if none were set
	gas, gasPrice := uint64(args.Gas), args.GasPrice.ToInt()
	if gas == 0 {
		gas = math.MaxUint64 / 2
	}
	if gasPrice.Sign() == 0 {
		gasPrice = new(big.Int).SetUint64(defaultGasPrice)
	}

	// Create new call message
	//msg := new(types.Transaction) //types.NewMessage(addr, args.To, 0, args.Value.ToInt(), gas, gasPrice, args.Data, false)
	//msg := &types.TransactionCall{types.NewTransaction(params.NonceAddOne, *args.To, args.Value.ToInt(), gas, gasPrice, args.Data, nil, nil, nil, 0, 0, "MAN", 0)}
	extra := make([]*types.ExtraTo_tr, 0)
	if len(args.ExtraTo) > 0 {
		var tmpExtra types.ExtraTo_tr
		for _, ar := range args.ExtraTo {
			tmpExtra.To_tr = ar.To2
			tmpExtra.Input_tr = ar.Input2
			tmpExtra.Value_tr = ar.Value2
			extra = append(extra, &tmpExtra)
		}
	}
	msg := &types.TransactionCall{types.NewTransactions(params.NonceAddOne, *args.To, args.Value.ToInt(), gas, gasPrice, args.Data, nil, nil, nil, extra, 0, 0, 0, "MAN", 0)}
	msg.SetFromLoad(addr)
	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	evm, vmError, err := s.b.GetEVM(ctx, msg, state, header, vmCfg)
	if err != nil {
		return nil, 0, false, err
	}
	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(core.GasPool).AddGas(math.MaxUint64)
	res, gas, failed, _, err := core.ApplyMessage(evm, msg, gp)
	if err := vmError(); err != nil {
		return nil, 0, false, err
	}
	return res, gas, failed, err
}

func ManArgsToCallArgs(manargs ManCallArgs) (args CallArgs, err error) {
	args.From, _ = base58.Base58DecodeToAddress(manargs.From)
	args.To = new(common.Address)
	if manargs.To != nil {
		args.To = new(common.Address)
		*args.To, err = base58.Base58DecodeToAddress(*manargs.To)
		if err != nil {
			return CallArgs{}, err
		}
	}
	if manargs.Currency == nil {
		return CallArgs{}, errors.New("missing required field 'currency'")
	}
	if !common.IsValidityManCurrency(*manargs.Currency) {
		return CallArgs{}, errors.New("invalidity currency")
	}
	args.Currency = new(string)
	*args.Currency = *manargs.Currency
	args.GasPrice = manargs.GasPrice
	args.Gas = manargs.Gas
	args.Value = manargs.Value
	args.Data = manargs.Data
	if len(manargs.ExtraTo) > 0 {
		extra := make([]*ExtraTo_Mx, 0)
		for _, ar := range manargs.ExtraTo {
			if ar.To2 != nil {
				tmp := *ar.To2
				tmp = strings.TrimSpace(tmp)
				tmExtra := new(ExtraTo_Mx)
				tmExtra.To2 = new(common.Address)
				*tmExtra.To2, err = base58.Base58DecodeToAddress(tmp)
				if err != nil {
					return CallArgs{}, err
				}
				tmExtra.Input2 = ar.Input2
				tmExtra.Value2 = ar.Value2
				extra = append(extra, tmExtra)
			}
		}
		args.ExtraTo = extra
	}

	return args, nil
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockChainAPI) Call(ctx context.Context, manargs ManCallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	args, err := ManArgsToCallArgs(manargs)
	if err != nil {
		return nil, err
	}
	result, _, _, err := s.doCall(ctx, args, blockNr, vm.Config{}, 5*time.Second)
	return (hexutil.Bytes)(result), err
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, manargs ManCallArgs) (hexutil.Uint64, error) {
	args, err := ManArgsToCallArgs(manargs)
	if err != nil {
		return 0, err
	}
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  uint64 = params.TxGas - 1
		hi  uint64
		cap uint64
	)
	if uint64(args.Gas) >= params.TxGas {
		hi = uint64(args.Gas)
	} else {
		//hi = params.MinGasLimit
		// Retrieve the current pending block to act as the gas ceiling
		block, err := s.b.BlockByNumber(ctx, rpc.LatestBlockNumber)
		if err != nil {
			return 0, err
		}
		hi = block.GasLimit()
	}
	cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) bool {
		args.Gas = hexutil.Uint64(gas)

		_, _, failed, err := s.doCall(ctx, args, rpc.LatestBlockNumber, vm.Config{}, 0)
		if err != nil || failed {
			return false
		}
		return true
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance or always failing transaction")
		}
	}
	//if len(args.ExtraTo) > 0{
	//	hi += 21000*uint64(len(args.ExtraTo))
	//}
	return hexutil.Uint64(hi), nil
}

// GetSelfLevel get self level from ca, including top node, buckets number and default.
func (s *PublicBlockChainAPI) GetSelfLevel() int {
	return ca.GetSelfLevel()
}

// GetSignAccounts get sign accounts form current block.
func (s *PublicBlockChainAPI) getSignAccountsByNumber1(ctx context.Context, blockNr rpc.BlockNumber) ([]common.VerifiedSign, common.Hash, error) {
	header, err := s.b.HeaderByNumber(ctx, blockNr)
	if header != nil {
		return header.SignAccounts(), header.Hash(), nil
	}
	return nil, common.Hash{}, err
}

func (s *PublicBlockChainAPI) GetSignAccountsByNumber(ctx context.Context, blockNr rpc.BlockNumber) ([]common.VerifiedSign1, error) {
	verSignList, blockHash, err := s.getSignAccountsByNumber1(ctx, blockNr)
	if err != nil {
		return nil, err
	}

	accounts := make([]common.VerifiedSign1, 0)
	for _, tmpverSign := range verSignList {
		depositAccount, err := s.b.GetDepositAccount(tmpverSign.Account, blockHash)
		if err != nil || (depositAccount == common.Address{}) {
			log.Debug("API", "GetSignAccountsByNumber", "get deposit account err", "sign account", tmpverSign.Account.Hex(), "err", err)
			continue
		}
		accounts = append(accounts, common.VerifiedSign1{
			Sign:     tmpverSign.Sign,
			Account:  base58.Base58EncodeToString(params.MAN_COIN, depositAccount),
			Validate: tmpverSign.Validate,
			Stock:    tmpverSign.Stock,
		})
	}
	return accounts, nil
}

func (s *PublicBlockChainAPI) getSignAccountsByHash1(ctx context.Context, hash common.Hash) ([]common.VerifiedSign, error) {
	block, err := s.b.GetBlock(ctx, hash)
	if block != nil {
		return block.SignAccounts(), nil
	}
	return nil, err
}
func (s *PublicBlockChainAPI) GetSignAccountsByHash(ctx context.Context, hash common.Hash) ([]common.VerifiedSign1, error) {
	verSignList, err := s.getSignAccountsByHash1(ctx, hash)
	if err != nil {
		return nil, err
	}
	accounts := make([]common.VerifiedSign1, 0)
	for _, tmpverSign := range verSignList {
		depositAccount, err := s.b.GetDepositAccount(tmpverSign.Account, hash)
		if err != nil || (depositAccount == common.Address{}) {
			log.Debug("API", "GetSignAccountsByHash", "get deposit account err", "sign account", tmpverSign.Account.Hex(), "err", err)
			continue
		}

		accounts = append(accounts, common.VerifiedSign1{
			Sign:     tmpverSign.Sign,
			Account:  base58.Base58EncodeToString(params.MAN_COIN, depositAccount),
			Validate: tmpverSign.Validate,
			Stock:    tmpverSign.Stock,
		})
	}
	return accounts, nil
}

func (s *PublicBlockChainAPI) ImportSuperBlock(ctx context.Context, filePath string) (common.Hash, error) {
	return s.b.ImportSuperBlock(ctx, filePath)
}

type NodeInfo struct {
	Account  string `json:"account"`
	Online   bool   `json:"online"`
	Position uint16 `json:"position"`
}

type TopologyStatus struct {
	LeaderReelect         bool       `json:"leader_reelect"`
	Validators            []NodeInfo `json:"validators"`
	BackupValidators      []NodeInfo `json:"backup_validators"`
	Miners                []NodeInfo `json:"miners"`
	ElectValidators       []NodeInfo `json:"elect_validators"`
	ElectBackupValidators []NodeInfo `json:"elect_backup_validators"`
}

func (s *PublicBlockChainAPI) GetTopologyStatusByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*TopologyStatus, error) {
	preBlockNr := blockNr
	if blockNr > 0 {
		preBlockNr -= 1
	}
	preState, preHeader, err := s.b.StateAndHeaderByNumber(ctx, preBlockNr)
	if preState == nil || preHeader == nil || err != nil {
		return nil, err
	}
	topologyGraph, err := matrixstate.GetTopologyGraph(preState)
	if err != nil {
		return nil, err
	}
	onlineState, err := matrixstate.GetElectOnlineState(preState)
	if err != nil {
		return nil, err
	}
	bcInterval, err := matrixstate.GetBroadcastInterval(preState)
	if err != nil {
		return nil, err
	}

	result := &TopologyStatus{}
	// 判断是否leader重选过
	if bcInterval.IsBroadcastNumber(uint64(blockNr)) || bcInterval.IsReElectionNumber(uint64(blockNr-1)) {
		result.LeaderReelect = false
	} else {
		curHeader, err := s.b.HeaderByNumber(ctx, blockNr)
		if curHeader == nil || err != nil {
			return nil, err
		}
		cmpHeader := preHeader
		cmpNumber := blockNr - 1
		for bcInterval.IsBroadcastNumber(uint64(cmpNumber)) || cmpHeader.IsSuperHeader() {
			if cmpNumber == 0 {
				return nil, errors.New("无对比区块")
			}
			cmpNumber--
			cmpHeader, err = s.b.HeaderByNumber(ctx, cmpNumber)
			if err != nil {
				return nil, err
			}
		}
		nextLeader := topologyGraph.FindNextValidator(cmpHeader.Leader)
		result.LeaderReelect = nextLeader != curHeader.Leader
	}

	// 拓扑图信息写入
	for _, node := range topologyGraph.NodeList {
		switch node.Type {
		case common.RoleValidator:
			result.Validators = append(result.Validators, NodeInfo{
				Account:  base58.Base58EncodeToString(params.MAN_COIN, node.Account),
				Online:   true,
				Position: node.Position,
			})
		case common.RoleBackupValidator:
			result.BackupValidators = append(result.BackupValidators, NodeInfo{
				Account:  base58.Base58EncodeToString(params.MAN_COIN, node.Account),
				Online:   true,
				Position: node.Position,
			})
		case common.RoleMiner:
			result.Miners = append(result.Miners, NodeInfo{
				Account:  base58.Base58EncodeToString(params.MAN_COIN, node.Account),
				Online:   true,
				Position: node.Position,
			})
		}
	}

	// 选举在线信息写入
	for _, node := range onlineState.ElectOnline {
		if topologyGraph.AccountIsInGraph(node.Account) {
			continue // 拓扑图中已存在的节点，过滤
		}

		online := node.Position != common.PosOffline
		switch node.Type {
		case common.RoleValidator:
			result.ElectValidators = append(result.ElectValidators, NodeInfo{
				Account:  base58.Base58EncodeToString(params.MAN_COIN, node.Account),
				Online:   online,
				Position: node.Position,
			})
		case common.RoleBackupValidator:
			result.ElectBackupValidators = append(result.ElectBackupValidators, NodeInfo{
				Account:  base58.Base58EncodeToString(params.MAN_COIN, node.Account),
				Online:   online,
				Position: node.Position,
			})
		}
	}
	return result, nil
}

// ExecutionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as transaction
// execution status, the amount of gas used and the return value
type ExecutionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}

// StructLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   error              `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

// formatLogs formats EVM returned structured logs for json output
func FormatLogs(logs []vm.StructLog) []StructLogRes {
	formatted := make([]StructLogRes, len(logs))
	for index, trace := range logs {
		formatted[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.Err,
		}
		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = fmt.Sprintf("%x", math.PaddedBigBytes(stackValue, 32))
			}
			formatted[index].Stack = &stack
		}
		if trace.Memory != nil {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
			}
			formatted[index].Memory = &memory
		}
		if trace.Storage != nil {
			storage := make(map[string]string)
			for i, storageValue := range trace.Storage {
				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
			}
			formatted[index].Storage = &storage
		}
	}
	return formatted
}

// rpcOutputBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func (s *PublicBlockChainAPI) rpcOutputBlock(b *types.Block, inclTx bool, fullTx bool) (map[string]interface{}, error) {
	head := b.Header() // copies the header once
	fields := map[string]interface{}{
		"number":     (*hexutil.Big)(head.Number),
		"hash":       b.Hash(),
		"parentHash": head.ParentHash,
		"nonce":      head.Nonce,
		"mixHash":    head.MixDigest,
		"sha3Uncles": head.UncleHash,
		//"logsBloom":         head.Bloom,
		"stateRoot":       head.Roots,
		"miner":           head.Coinbase,
		"difficulty":      (*hexutil.Big)(head.Difficulty),
		"totalDifficulty": (*hexutil.Big)(s.b.GetTd(b.Hash())),
		"extraData":       hexutil.Bytes(head.Extra),
		"size":            hexutil.Uint64(b.Size()),
		"gasLimit":        hexutil.Uint64(head.GasLimit),
		"gasUsed":         hexutil.Uint64(head.GasUsed),
		"timestamp":       (*hexutil.Big)(head.Time),
		//"transactionsRoot":  head.TxHash,
		//"receiptsRoot":      head.ReceiptHash,
		"leader":            head.Leader,
		"elect":             head.Elect,
		"nettopology":       head.NetTopology,
		"signatures":        head.Signatures,
		"version":           string(head.Version),
		"versionSignatures": head.VersionSignatures,
		"vrfvalue":          hexutil.Bytes(head.VrfValue),
	}

	if inclTx {
		formatTx := func(tx types.SelfTransaction, cointy string) (interface{}, error) {
			return tx.Hash(), nil
		}

		if fullTx {
			formatTx = func(tx types.SelfTransaction, cointy string) (interface{}, error) {
				return newRPCTransactionFromBlockHash(b, tx.Hash(), cointy), nil
			}
		}

		currencyFields := make(map[string]interface{})
		for _, curr := range b.Currencies() {

			txs := curr.Transactions.GetTransactions()
			transactions := make([]interface{}, len(txs))
			var err error
			for i, tx := range txs {
				if transactions[i], err = formatTx(tx, curr.CurrencyName); err != nil {
					return nil, err
				}
			}
			currencyFields[curr.CurrencyName] = transactions
		}
		fields["transactions"] = currencyFields
	}

	uncles := b.Uncles()
	uncleHashes := make([]common.Hash, len(uncles))
	for i, uncle := range uncles {
		uncleHashes[i] = uncle.Hash()
	}
	fields["uncles"] = uncleHashes

	return fields, nil
}

/************************************************************/
func (s *PublicBlockChainAPI) rpcOutputBlock1(b *types.Block, inclTx bool, fullTx bool) (map[string]interface{}, error) {
	head := b.Header() // copies the header once
	Coinbase1 := base58.Base58EncodeToString(params.MAN_COIN, head.Coinbase)
	Leader1 := base58.Base58EncodeToString(params.MAN_COIN, head.Leader)
	//head.NetTopology
	NetTopology1 := new(common.NetTopology1)
	listNetTopolog := make([]common.NetTopologyData1, 0)
	for _, addr := range head.NetTopology.NetTopologyData {
		tmpstruct := new(common.NetTopologyData1)
		tmpstruct.Account = base58.Base58EncodeToString(params.MAN_COIN, addr.Account)
		tmpstruct.Position = addr.Position
		listNetTopolog = append(listNetTopolog, *tmpstruct)
	}
	NetTopology1.Type = head.NetTopology.Type
	NetTopology1.NetTopologyData = append(NetTopology1.NetTopologyData, listNetTopolog...)

	//head.Elect
	listElect1 := make([]common.Elect1, 0)
	for _, elect := range head.Elect {
		tmpElect1 := new(common.Elect1)
		tmpElect1.Type = elect.Type
		tmpElect1.Account = base58.Base58EncodeToString(params.MAN_COIN, elect.Account)
		tmpElect1.Stock = elect.Stock
		tmpElect1.VIP = elect.VIP
		listElect1 = append(listElect1, *tmpElect1)
	}

	fields := map[string]interface{}{
		"number":     (*hexutil.Big)(head.Number),
		"hash":       b.Hash(),
		"signHash":   b.HashNoSignsAndNonce(),
		"parentHash": head.ParentHash,
		"nonce":      head.Nonce,
		"mixHash":    head.MixDigest,
		"sha3Uncles": head.UncleHash,
		//"logsBloom":        head.Bloom,			//BB?
		"stateRoot":       head.Roots,
		"sharding":        head.Sharding,
		"miner":           Coinbase1,
		"difficulty":      (*hexutil.Big)(head.Difficulty),
		"totalDifficulty": (*hexutil.Big)(s.b.GetTd(b.Hash())),
		"extraData":       hexutil.Bytes(head.Extra),
		"size":            hexutil.Uint64(b.Size()),
		"gasLimit":        hexutil.Uint64(head.GasLimit),
		"gasUsed":         hexutil.Uint64(head.GasUsed),
		"timestamp":       (*hexutil.Big)(head.Time),
		//"transactionsRoot": head.TxHash,
		//"receiptsRoot":     head.ReceiptHash,
		"leader":      Leader1,
		"elect":       listElect1,
		"nettopology": NetTopology1,
		"signatures":  head.Signatures,
		"version":     hexutil.Bytes(head.Version),
		"VrfValue":    hexutil.Bytes(head.VrfValue),
	}
	if manversion.VersionCmp(string(head.Version), manversion.VersionAIMine) >= 0 {
		fields["AIHash"] = head.AIHash
	}
	if inclTx {
		formatTx := func(tx types.SelfTransaction, cointy string) (interface{}, error) {
			return tx.Hash(), nil
		}

		if fullTx {
			formatTx = func(tx types.SelfTransaction, cointy string) (interface{}, error) {
				return newRPCTransactionFromBlockHash(b, tx.Hash(), cointy), nil
			}
		}

		currencyFields := make(map[string]interface{})
		for _, curr := range b.Currencies() {

			txs := curr.Transactions.GetTransactions()
			transactions := make([]interface{}, len(txs))
			var err error
			for i, tx := range txs {
				if transactions[i], err = formatTx(tx, curr.CurrencyName); err != nil {
					return nil, err
				}
			}
			currencyFields[curr.CurrencyName] = transactions
		}
		fields["transactions"] = currencyFields
	}

	uncles := b.Uncles()
	uncleHashes := make([]common.Hash, len(uncles))
	for i, uncle := range uncles {
		uncleHashes[i] = uncle.Hash()
	}
	fields["uncles"] = uncleHashes

	return fields, nil
}

//
type RPCTransaction1 struct {
	BlockHash        common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big   `json:"blockNumber"`
	From             string         `json:"from"`
	Gas              hexutil.Uint64 `json:"gas"`
	GasPrice         *hexutil.Big   `json:"gasPrice"`
	Hash             common.Hash    `json:"hash"`
	Input            hexutil.Bytes  `json:"input"`
	Nonce            hexutil.Uint64 `json:"nonce"`
	To               *string        `json:"to"`
	TransactionIndex hexutil.Uint   `json:"transactionIndex"`
	Value            *hexutil.Big   `json:"value"`
	V                *hexutil.Big   `json:"v"`
	R                *hexutil.Big   `json:"r"`
	S                *hexutil.Big   `json:"s"`
	TxEnterType      byte           `json:"TxEnterType"`
	IsEntrustTx      bool           `json:"IsEntrustTx"`
	Currency         string         `json:"Currency"`
	CommitTime       hexutil.Uint64 `json:"CommitTime"`
	MatrixType       byte           `json:"matrixType"`
	ExtraTo          []*ExtraTo_Mx1 `json:"extra_to"`
}

func RPCTransactionToString(data *RPCTransaction) *RPCTransaction1 {
	result := &RPCTransaction1{
		BlockHash:        data.BlockHash,
		BlockNumber:      data.BlockNumber,
		Gas:              data.Gas,
		GasPrice:         data.GasPrice,
		Hash:             data.Hash,
		Input:            data.Input,
		Nonce:            data.Nonce,
		TransactionIndex: data.TransactionIndex,
		Value:            data.Value,
		V:                data.V,
		R:                data.R,
		S:                data.S,
		TxEnterType:      data.TxEnterType,
		IsEntrustTx:      data.IsEntrustTx,
		Currency:         data.Currency,
		MatrixType:       data.MatrixType,
		CommitTime:       data.CommitTime,
	}
	//内部发送的交易没有币种，默认为MAN
	if data.Currency == "" {
		data.Currency = params.MAN_COIN
	}
	result.From = base58.Base58EncodeToString(data.Currency, data.From)
	if data.To != nil {
		result.To = new(string)
		*result.To = base58.Base58EncodeToString(data.Currency, *data.To)
	}

	if len(data.ExtraTo) > 0 {
		extra := make([]*ExtraTo_Mx1, 0)
		for _, ar := range data.ExtraTo {
			if ar.To2 != nil {
				tmExtra := new(ExtraTo_Mx1)
				tmExtra.To2 = new(string)
				*tmExtra.To2 = base58.Base58EncodeToString(data.Currency, *ar.To2)
				tmExtra.Input2 = ar.Input2
				tmExtra.Value2 = ar.Value2
				extra = append(extra, tmExtra)
			}
		}
		result.ExtraTo = extra
	}

	return result
}

/************************************************************/
// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        common.Hash     `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex hexutil.Uint    `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
	TxEnterType      byte            `json:"TxEnterType"`
	IsEntrustTx      bool            `json:"IsEntrustTx"`
	Currency         string          `json:"Currency"`
	CommitTime       hexutil.Uint64  `json:"CommitTime"`
	MatrixType       byte            `json:"matrixType"`
	ExtraTo          []*ExtraTo_Mx   `json:"extra_to"`
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCTransaction(tx types.SelfTransaction, blockHash common.Hash, blockNumber uint64, index uint64) *RPCTransaction {
	var signer types.Signer //= types.FrontierSigner{}
	//if tx.Protected() {
	signer = types.NewEIP155Signer(tx.ChainId())
	//}

	var from common.Address

	if tx.GetMatrixType() == common.ExtraUnGasMinerTxType || tx.GetMatrixType() == common.ExtraUnGasValidatorTxType ||
		tx.GetMatrixType() == common.ExtraUnGasInterestTxType || tx.GetMatrixType() == common.ExtraUnGasTxsType || tx.GetMatrixType() == common.ExtraUnGasLotteryTxType {
		from = tx.From()
	} else {
		from, _ = types.Sender(signer, tx)
	}
	v, r, s := tx.RawSignatureValues()

	result := &RPCTransaction{
		From:        from,
		Gas:         hexutil.Uint64(tx.Gas()),
		GasPrice:    (*hexutil.Big)(tx.GasPrice()),
		Hash:        tx.Hash(),
		Input:       hexutil.Bytes(tx.Data()),
		Nonce:       hexutil.Uint64(tx.Nonce()),
		To:          tx.To(),
		Value:       (*hexutil.Big)(tx.Value()),
		V:           (*hexutil.Big)(v),
		R:           (*hexutil.Big)(r),
		S:           (*hexutil.Big)(s),
		TxEnterType: tx.TxType(),
		IsEntrustTx: tx.IsEntrustTx(),
		Currency:    tx.GetTxCurrency(),
		MatrixType:  tx.GetMatrixType(),
		CommitTime:  hexutil.Uint64(tx.GetCreateTime()),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = hexutil.Uint(index)
	}

	extra := tx.GetMatrix_EX()
	for _, ext := range extra {
		for _, e := range ext.ExtraTo {
			b := hexutil.Bytes(e.Payload)
			//b = nil //屏蔽input
			result.ExtraTo = append(result.ExtraTo, &ExtraTo_Mx{
				To2:    e.Recipient,
				Input2: &b,
				Value2: (*hexutil.Big)(e.Amount),
			})
		}
	}
	//result.Input = nil //屏蔽input
	return result
}

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx types.SelfTransaction) *RPCTransaction {
	return newRPCTransaction(tx, common.Hash{}, 0, 0)
}

// newRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockIndex(b *types.Block, index uint64, cointy string) *RPCTransaction1 {
	rpcTrans := newRPCTransactionFromBlockIndex1(b, index, cointy)
	if rpcTrans != nil {
		return RPCTransactionToString(rpcTrans)
	}
	return nil
}

func newRPCTransactionFromBlockIndex1(b *types.Block, index uint64, cointy string) *RPCTransaction {
	txs := make(types.SelfTransactions, 0)
	for _, curr := range b.Currencies() {
		if curr.CurrencyName == cointy {
			txs = append(txs, curr.Transactions.GetTransactions()...)
		}
	}
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCTransaction(txs[index], b.Hash(), b.NumberU64(), index)
}

// newRPCRawTransactionFromBlockIndex returns the bytes of a transaction given a block and a transaction index.
func newRPCRawTransactionFromBlockIndex(b *types.Block, index uint64, cointy string) hexutil.Bytes {
	txs := make(types.SelfTransactions, 0)
	for _, curr := range b.Currencies() {
		if curr.CurrencyName == cointy {
			txs = append(txs, curr.Transactions.GetTransactions()...)
		}
	}
	if index >= uint64(len(txs)) {
		return nil
	}
	blob, _ := rlp.EncodeToBytes(txs[index])
	return blob
}

// newRPCTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockHash(b *types.Block, hash common.Hash, cointy string) *RPCTransaction1 {
	txs := make(types.SelfTransactions, 0)
	for _, curr := range b.Currencies() {
		if curr.CurrencyName == cointy {
			txs = append(txs, curr.Transactions.GetTransactions()...)
		}
	}
	for idx, tx := range txs {
		if tx.Hash() == hash {
			return newRPCTransactionFromBlockIndex(b, uint64(idx), cointy)
		}
	}
	return nil
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, nonceLock}
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		txcount := 0
		for _, curr := range block.Currencies() {
			txcount += len(curr.Transactions.GetTransactions())
		}
		n := hexutil.Uint(txcount)
		return &n
	}
	return nil
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		txcount := 0
		for _, curr := range block.Currencies() {
			txcount += len(curr.Transactions.GetTransactions())
		}
		n := hexutil.Uint(txcount)
		return &n
	}
	return nil
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint, cointy string) *RPCTransaction1 {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index), cointy)
	}
	return nil
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint, cointy string) *RPCTransaction1 {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index), cointy)
	}
	return nil
}

// GetRawTransactionByBlockNumberAndIndex returns the bytes of the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint, cointy string) hexutil.Bytes {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCRawTransactionFromBlockIndex(block, uint64(index), cointy)
	}
	return nil
}

// GetRawTransactionByBlockHashAndIndex returns the bytes of the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint, cointy string) hexutil.Bytes {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCRawTransactionFromBlockIndex(block, uint64(index), cointy)
	}
	return nil
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, strAddress string, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	cointype, err := getCoinFromManAddress(strAddress)
	if err != nil {
		return nil, err
	}
	address, err := base58.Base58DecodeToAddress(strAddress)
	if err != nil {
		return nil, err
	}
	nonce := state.GetNonce(cointype, address)
	return (*hexutil.Uint64)(&nonce), state.Error()
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) getTransactionByHash1(ctx context.Context, hash common.Hash) *RPCTransaction {
	// Try to return an already finalized transaction
	if tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash); tx != nil {
		return newRPCTransaction(tx, blockHash, blockNumber, index)
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.b.GetPoolTransaction(hash); tx != nil {
		return newRPCPendingTransaction(tx)
	}
	// Transaction unknown, return as such
	return nil
}

//
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) *RPCTransaction1 {
	rpcTrans := s.getTransactionByHash1(ctx, hash)
	if rpcTrans != nil {
		return RPCTransactionToString(rpcTrans)
	}
	return nil
}

// GetRawTransactionByHash returns the bytes of the transaction for the given hash.
func (s *PublicTransactionPoolAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	var tx types.SelfTransaction

	// Retrieve a finalized transaction, or a pooled otherwise
	if tx, _, _, _ = rawdb.ReadTransaction(s.b.ChainDb(), hash); tx == nil {
		if tx = s.b.GetPoolTransaction(hash); tx == nil {
			// Transaction not found anywhere, abort
			return nil, nil
		}
	}
	// Serialize to RLP and return
	return rlp.EncodeToBytes(tx)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash)
	if tx == nil {
		return nil, nil
	}
	coinreceipts, err := s.b.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	var receipts types.Receipts
	for _, cr := range coinreceipts {
		if cr.CoinType == tx.GetTxCurrency() {
			receipts = cr.Receiptlist
		}
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]

	var signer types.Signer //= types.FrontierSigner{}
	//if tx.Protected() {
	signer = types.NewEIP155Signer(tx.ChainId())
	//}
	from, _ := types.Sender(signer, tx)

	fields := map[string]interface{}{
		"Currency":          tx.GetTxCurrency(),
		"blockHash":         blockHash,
		"blockNumber":       hexutil.Uint64(blockNumber),
		"transactionHash":   hash,
		"transactionIndex":  hexutil.Uint64(index),
		"from":              from,
		"to":                tx.To(),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
	}
	fields["from"] = base58.Base58EncodeToString(tx.GetTxCurrency(), from)
	if tx.To() != nil {
		fields["to"] = base58.Base58EncodeToString(tx.GetTxCurrency(), *tx.To())
	}
	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = hexutil.Bytes(receipt.PostState)
	} else {
		fields["status"] = hexutil.Uint(receipt.Status)
	}
	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = base58.Base58EncodeToString(tx.GetTxCurrency(), receipt.ContractAddress)
	}
	return fields, nil
}

// sign is a helper function that signs a transaction with the private key of the given address.
func (s *PublicTransactionPoolAPI) sign(strAddr string, tx types.SelfTransaction) (types.SelfTransaction, error) {
	addr, err := base58.Base58DecodeToAddress(strAddr)
	if err != nil {
		return nil, err
	}

	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: addr}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	// Request the wallet to sign the transaction
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	return wallet.SignTx(account, tx, chainID)
}

//
type ExtraTo_Mx struct {
	To2    *common.Address `json:"to"`
	Value2 *hexutil.Big    `json:"value"`
	Input2 *hexutil.Bytes  `json:"input"`
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	From     common.Address  `json:"from"`
	Currency string          `json:"currency"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data        *hexutil.Bytes `json:"data"`
	Input       *hexutil.Bytes `json:"input"`
	V           *hexutil.Big   `json:"v"`
	R           *hexutil.Big   `json:"r"`
	S           *hexutil.Big   `json:"s"`
	TxType      byte           `json:"txType"`     //
	LockHeight  uint64         `json:"lockHeight"` //
	IsEntrustTx byte           `json:"isEntrustTx"`
	CommitTime  uint64         `json:"commitTime"`
	ExtraTo     []*ExtraTo_Mx  `json:"extra_to"` //
}

type ExtraTo_Mx1 struct {
	To2    *string        `json:"to"`
	Value2 *hexutil.Big   `json:"value"`
	Input2 *hexutil.Bytes `json:"input"`
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs1 struct {
	From     string          `json:"from"`
	To       *string         `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data        *hexutil.Bytes `json:"data"`
	Input       *hexutil.Bytes `json:"input"`
	V           *hexutil.Big   `json:"v"`
	R           *hexutil.Big   `json:"r"`
	S           *hexutil.Big   `json:"s"`
	Currency    *string        `json:"currency"`
	TxType      byte           `json:"txType"`     //
	LockHeight  uint64         `json:"lockHeight"` //
	IsEntrustTx byte           `json:"isEntrustTx"`
	CommitTime  uint64         `json:"commitTime"`
	ExtraTo     []*ExtraTo_Mx1 `json:"extra_to"` //
}

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
		//
		if len(args.ExtraTo) > 0 {
			*(*uint64)(args.Gas) = 21000*uint64(len(args.ExtraTo)) + 21000
		} else {
			*(*uint64)(args.Gas) = 21000
		}
	}
	state, err := b.GetState()
	if err != nil {
		return err
	}
	price, err := matrixstate.GetTxpoolGasLimit(state)
	if err != nil {
		return err
	}

	args.GasPrice = (*hexutil.Big)(price)

	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Nonce == nil {
		nonce, err := b.GetPoolNonce(args.Currency, ctx, args.From)
		if err != nil {
			return err
		}
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`Both "data" and "input" are set and not equal. Please use "input" to pass transaction call data.`)
	}
	if args.To == nil {
		// Contract creation
		var input []byte
		if args.Data != nil {
			input = *args.Data
		} else if args.Input != nil {
			input = *args.Input
		}
		if len(input) == 0 {
			return errors.New(`contract creation without any data provided`)
		}
	}
	return nil
}

func (args *SendTxArgs) toTransaction() *types.Transaction {
	var input []byte
	if args.Data != nil {
		input = *args.Data
	} else if args.Input != nil {
		input = *args.Input
	}
	if args.To == nil {
		return types.NewContractCreation(uint64(*args.Nonce), (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input, (*big.Int)(args.V), (*big.Int)(args.R), (*big.Int)(args.S), 0, args.IsEntrustTx, args.Currency, args.CommitTime)
	}

	//
	txtr := make([]*types.ExtraTo_tr, 0)
	if len(args.ExtraTo) > 0 {
		for _, extra := range args.ExtraTo {
			tmp := new(types.ExtraTo_tr)
			va := extra.Value2
			if va == nil {
				va = (*hexutil.Big)(big.NewInt(0))
			}
			tmp.To_tr = extra.To2
			tmp.Value_tr = va
			tmp.Input_tr = extra.Input2
			txtr = append(txtr, tmp)
		}
	}
	return types.NewTransactions(uint64(*args.Nonce), *args.To, (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input, (*big.Int)(args.V), (*big.Int)(args.R), (*big.Int)(args.S), txtr, args.LockHeight, args.TxType, args.IsEntrustTx, args.Currency, args.CommitTime)

}

// submitTransaction is a helper function that submits tx to txPool and logs a message.
func submitTransaction(ctx context.Context, b Backend, tx types.SelfTransaction) (common.Hash, error) {
	if err := b.SendTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}
	if tx.To() == nil {
		signer := types.MakeSigner(b.ChainConfig(), b.CurrentBlock().Number())
		from, err := types.Sender(signer, tx)
		if err != nil {
			return common.Hash{}, err
		}
		addr := crypto.CreateAddress(from, tx.Nonce())
		log.Info("Submitted contract creation", "fullhash", tx.Hash().Hex(), "contract", addr.Hex())
	} else {
		//log.Info("Submitted transaction", "fullhash", tx.Hash().Hex(), "recipient", tx.To())
	}
	//log.Info("file api","func submitTransaction",tx.Hash().String())
	return tx.Hash(), nil
}

func CheckCrc8(strData string) bool {
	Crc := strData[len(strData)-1 : len(strData)]
	reCrc := crc8.CalCRC8([]byte(strData[0 : len(strData)-1]))
	ModCrc := reCrc % 58
	ret := base58.EncodeInt(ModCrc)
	if Crc != ret {
		return false
	}
	return true
}
func CheckCurrency(strData string) bool {
	currency := strings.Split(strData, ".")[0]
	return common.IsValidityManCurrency(currency)
}
func CheckFormat(strData string) bool {
	if !strings.Contains(strData, ".") {
		return false
	}
	return true
}
func CheckParams(strData string) error {
	strData = strings.TrimSpace(strData)
	if !CheckFormat(strData) {
		return errors.New("format error")
	}
	if !CheckCrc8(strData) {
		return errors.New("CRC error")
	}
	if !CheckCurrency(strData) {
		return errors.New("currency error")
	}
	return nil
}
func StrArgsToByteArgs(args1 SendTxArgs1) (args SendTxArgs, err error) {
	if args1.From != "" {
		from := args1.From
		err = CheckParams(from)
		if err != nil {
			return SendTxArgs{}, err
		}
		args.Currency = strings.Split(args1.From, ".")[0]
		args.From, err = base58.Base58DecodeToAddress(from)
		if err != nil {
			return SendTxArgs{}, err
		}
	}
	if args1.Currency != nil {
		args.Currency = *args1.Currency
		args.Currency = strings.TrimSpace(args.Currency)
	}
	if !common.IsValidityManCurrency(args.Currency) {
		return SendTxArgs{}, errors.New("invalid currency")
	}
	if args1.To != nil {
		to := *args1.To
		to = strings.TrimSpace(to)
		tCurrency := strings.Split(to, ".")[0]
		if args.Currency != tCurrency {
			return SendTxArgs{}, errors.New("different currency")
		}
		err = CheckParams(to)
		if err != nil {
			return SendTxArgs{}, err
		}
		args.To = new(common.Address)
		*args.To, err = base58.Base58DecodeToAddress(to)
		if err != nil {
			return SendTxArgs{}, err
		}
	}
	if args1.V != nil {
		args.V = args1.V
	}
	if args1.R != nil {
		args.R = args1.R
	}
	if args1.S != nil {
		args.S = args1.S
	}
	args.Gas = args1.Gas
	args.GasPrice = args1.GasPrice
	args.Value = args1.Value
	args.Nonce = args1.Nonce
	args.Data = args1.Data
	args.Input = args1.Input
	args.TxType = args1.TxType
	args.LockHeight = args1.LockHeight
	args.CommitTime = args1.CommitTime
	args.IsEntrustTx = args1.IsEntrustTx
	if len(args1.ExtraTo) > 0 { //扩展交易中的to属性不填写则删掉这个扩展交易
		extra := make([]*ExtraTo_Mx, 0)
		for _, ar := range args1.ExtraTo {
			if ar.To2 != nil {
				//extra = append(extra, ar)
				tmp := *ar.To2
				tmp = strings.TrimSpace(tmp)
				tCurrency := strings.Split(tmp, ".")[0]
				if args.Currency != tCurrency {
					return SendTxArgs{}, errors.New("different currency")
				}
				err = CheckParams(tmp)
				if err != nil {
					return SendTxArgs{}, err
				}
				tmExtra := new(ExtraTo_Mx)
				tmExtra.To2 = new(common.Address)
				*tmExtra.To2, err = base58.Base58DecodeToAddress(tmp)
				if err != nil {
					return SendTxArgs{}, err
				}
				tmExtra.Input2 = ar.Input2
				tmExtra.Value2 = ar.Value2
				extra = append(extra, tmExtra)
			}
		}
		args.ExtraTo = extra
	}
	return args, nil
}

// SendTransaction creates a transaction for the given argument, sign it and submit it to the
// transaction pool.
func (s *PublicTransactionPoolAPI) SendTransaction(ctx context.Context, args1 SendTxArgs1) (common.Hash, error) {
	//from字段格式: 2-8长度币种（大写）+ “.”+ 以太坊地址的base58编码 + crc8/58
	if args1.TxType == common.ExtraBroadTxType {
		return common.Hash{}, errors.New("TxType can not be set 1")
	}
	var args SendTxArgs
	args, err := StrArgsToByteArgs(args1)
	if err != nil {
		return common.Hash{}, err
	}
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}
	if args.Nonce == nil {
		// Hold the addresse's mutex around signing to prevent concurrent assignment of
		// the same nonce to multiple accounts.
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	} else { // add else
		nc1 := params.NonceAddOne
		nc := uint64(*args.Nonce)
		if nc < nc1 {
			err = errors.New("Nonce Wrongful")
			return common.Hash{}, err
		}
	}
	//
	if len(args.ExtraTo) > 0 { //扩展交易中的to和input属性不填写则删掉这个扩展交易
		extra := make([]*ExtraTo_Mx, 0)
		for _, ar := range args.ExtraTo {
			if ar.To2 != nil || ar.Input2 != nil {
				extra = append(extra, ar)
			}
		}
		args.ExtraTo = extra
	}
	// Set some sanity defaults and terminate on failure
	if err := args.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	// Assemble the transaction and sign with the wallet
	tx := args.toTransaction()
	var chainID *big.Int
	if config := s.b.ChainConfig(); config.IsEIP155(s.b.CurrentBlock().Number()) {
		chainID = config.ChainId
	}
	//tx.Currency = args.Currency
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}
	//Currency := args.Currency //币种
	//signed.SetTxCurrency(Currency)
	return submitTransaction(ctx, s.b, signed)
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, args1 SendTxArgs1) (common.Hash, error) {
	if args1.TxType == common.ExtraBroadTxType {
		return common.Hash{}, errors.New("TxType can not be set 1")
	}
	var args SendTxArgs
	args, err := StrArgsToByteArgs(args1)
	if err != nil {
		return common.Hash{}, err
	}
	tx := args.toTransaction()
	return submitTransaction(ctx, s.b, tx)
}
func (s *PublicTransactionPoolAPI) SendRawTransaction_old(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	//tx.Mtype = true
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	return submitTransaction(ctx, s.b, tx)
}

// Sign calculates an ECDSA signature for:
// keccack256("\x19Matrix Signed Message:\n" + len(message) + message).
//
// Note, the produced signature conforms to the secp256k1 curve R, S and V values,
// where the V value will be 27 or 28 for legacy reasons.
//
// The account associated with addr must be unlocked.
//
// https://github.com/MatrixAINetwork/wiki/wiki/JSON-RPC#man_sign
func (s *PublicTransactionPoolAPI) Sign(strAddr string, data hexutil.Bytes) (hexutil.Bytes, error) {
	addr, err := base58.Base58DecodeToAddress(strAddr)
	if err != nil {
		return nil, err
	}
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: addr}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	// Sign the requested hash with the wallet
	signature, err := wallet.SignHash(account, signHash(data))
	if err == nil {
		signature[64] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	}
	return signature, err
}

// SignTransactionResult represents a RLP encoded signed transaction.
type SignTransactionResult struct {
	Raw hexutil.Bytes         `json:"raw"`
	Tx  types.SelfTransaction `json:"tx"`
}

// SignTransaction will sign the given transaction with the from account.
// The node needs to have the private key of the account corresponding with
// the given from address and it needs to be unlocked.
func (s *PublicTransactionPoolAPI) SignTransaction(ctx context.Context, args1 SendTxArgs1) (*SignTransactionResult, error) {
	var args SendTxArgs
	args, err := StrArgsToByteArgs(args1)
	if err != nil {
		return nil, err
	}
	if args.Gas == nil {
		return nil, fmt.Errorf("gas not specified")
	}
	if args.GasPrice == nil {
		return nil, fmt.Errorf("gasPrice not specified")
	}
	if args.Nonce == nil {
		return nil, fmt.Errorf("nonce not specified")
	}
	if err := args.setDefaults(ctx, s.b); err != nil {
		return nil, err
	}
	tx, err := s.sign(args1.From, args.toTransaction())
	if err != nil {
		return nil, err
	}
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	return &SignTransactionResult{data, tx}, nil
}

// PendingTransactions returns the transactions that are in the transaction pool and have a from address that is one of
// the accounts this node manages.
func (s *PublicTransactionPoolAPI) PendingTransactions() ([]*RPCTransaction, error) {
	pending, err := s.b.GetPoolTransactions()
	if err != nil {
		return nil, err
	}

	transactions := make([]*RPCTransaction, 0, len(pending))
	for _, tx := range pending {
		var signer types.Signer //= types.HomesteadSigner{}
		//if tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainId())
		//}
		from, _ := types.Sender(signer, tx)
		if _, err := s.b.AccountManager().Find(accounts.Account{Address: from}); err == nil {
			transactions = append(transactions, newRPCPendingTransaction(tx))
		}
	}
	return transactions, nil
}

// Resend accepts an existing transaction and a new gas price and limit. It will remove
// the given transaction from the pool and reinsert it with the new gas price and limit.
func (s *PublicTransactionPoolAPI) Resend(ctx context.Context, sendArgs1 SendTxArgs1, gasPrice *hexutil.Big, gasLimit *hexutil.Uint64) (common.Hash, error) {
	var sendArgs SendTxArgs
	sendArgs, err := StrArgsToByteArgs(sendArgs1)
	if err != nil {
		return common.Hash{}, err
	}
	if sendArgs.Nonce == nil {
		return common.Hash{}, fmt.Errorf("missing transaction nonce in transaction spec")
	}
	if err := sendArgs.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	matchTx := sendArgs.toTransaction()
	pending, err := s.b.GetPoolTransactions()
	if err != nil {
		return common.Hash{}, err
	}

	for _, p := range pending {
		var signer types.Signer //= types.HomesteadSigner{}
		//if p.Protected() {
		signer = types.NewEIP155Signer(p.ChainId())
		//}
		wantSigHash := signer.Hash(matchTx)

		if pFrom, err := types.Sender(signer, p); err == nil && pFrom == sendArgs.From && signer.Hash(p) == wantSigHash {
			// Match. Re-sign and send the transaction.
			if gasPrice != nil && (*big.Int)(gasPrice).Sign() != 0 {
				sendArgs.GasPrice = gasPrice
			}
			if gasLimit != nil && *gasLimit != 0 {
				sendArgs.Gas = gasLimit
			}
			Currency := strings.Split(sendArgs1.From, ".")[0] //币种
			strFrom := base58.Base58EncodeToString(Currency, sendArgs.From)
			signedTx, err := s.sign(strFrom, sendArgs.toTransaction())
			if err != nil {
				return common.Hash{}, err
			}
			if err = s.b.SendTx(ctx, signedTx); err != nil {
				return common.Hash{}, err
			}
			return signedTx.Hash(), nil
		}
	}

	return common.Hash{}, fmt.Errorf("Transaction %#x not found", matchTx.Hash())
}

// PublicDebugAPI is the collection of Matrix APIs exposed over the public
// debugging endpoint.
type PublicDebugAPI struct {
	b Backend
}

// NewPublicDebugAPI creates a new API definition for the public debug methods
// of the Matrix service.
func NewPublicDebugAPI(b Backend) *PublicDebugAPI {
	return &PublicDebugAPI{b: b}
}

func (api *PublicDebugAPI) GetAllChainInfo() map[string]interface{} {
	result := make(map[string]interface{})
	result["chainId"] = api.b.ChainConfig().ChainId
	result["ByzantiumBlock"] = api.b.ChainConfig().ByzantiumBlock
	result["EIP155Block"] = api.b.ChainConfig().EIP155Block
	result["EIP158Block"] = api.b.ChainConfig().EIP158Block
	result["NetworkId"] = api.b.NetWorkID()
	result["SyncMode"] = api.b.SyncMode()
	result["Genesis"] = api.b.Genesis().Hash()
	result["PeerCount"] = api.b.NetRPCService().PeerCount()
	result["LastBlockNumber"] = api.b.CurrentBlock().NumberU64()
	result["LastBlockHash"] = api.b.CurrentBlock().Hash()
	return result
}

// GetBlockRlp retrieves the RLP encoded for of a single block.
func (api *PublicDebugAPI) GetBlockRlp(ctx context.Context, number uint64) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	encoded, err := rlp.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", encoded), nil
}

// PrintBlock retrieves a block and returns its pretty printed form.
func (api *PublicDebugAPI) PrintBlock(ctx context.Context, number uint64) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return spew.Sdump(block), nil
}

// SeedHash retrieves the seed hash of a block.
func (api *PublicDebugAPI) SeedHash(ctx context.Context, number uint64) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return fmt.Sprintf("0x%x", manash.SeedHash(number)), nil
}

// PrivateDebugAPI is the collection of Matrix APIs exposed over the private
// debugging endpoint.
type PrivateDebugAPI struct {
	b Backend
}

// NewPrivateDebugAPI creates a new API definition for the private debug methods
// of the Matrix service.
func NewPrivateDebugAPI(b Backend) *PrivateDebugAPI {
	return &PrivateDebugAPI{b: b}
}

// ChaindbProperty returns leveldb properties of the chain database.
func (api *PrivateDebugAPI) ChaindbProperty(property string) (string, error) {
	ldb, ok := api.b.ChainDb().(interface {
		LDB() *leveldb.DB
	})
	if !ok {
		return "", fmt.Errorf("chaindbProperty does not work for memory databases")
	}
	if property == "" {
		property = "leveldb.stats"
	} else if !strings.HasPrefix(property, "leveldb.") {
		property = "leveldb." + property
	}
	return ldb.LDB().GetProperty(property)
}

func (api *PrivateDebugAPI) ChaindbCompact() error {
	ldb, ok := api.b.ChainDb().(interface {
		LDB() *leveldb.DB
	})
	if !ok {
		return fmt.Errorf("chaindbCompact does not work for memory databases")
	}
	for b := byte(0); b < 255; b++ {
		log.Info("Compacting chain database", "range", fmt.Sprintf("0x%0.2X-0x%0.2X", b, b+1))
		err := ldb.LDB().CompactRange(util.Range{Start: []byte{b}, Limit: []byte{b + 1}})
		if err != nil {
			log.Error("Database compaction failed", "err", err)
			return err
		}
	}
	return nil
}

// SetHead rewinds the head of the blockchain to a previous block.
func (api *PrivateDebugAPI) SetHead(number hexutil.Uint64) {
	api.b.SetHead(uint64(number))
}

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	net            *p2p.Server
	networkVersion uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net *p2p.Server, networkVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{net, networkVersion}
}

// Listening returns an indication if the node is listening for network connections.
func (s *PublicNetAPI) Listening() bool {
	return true // always listening
}

// PeerCount returns the number of connected peers
func (s *PublicNetAPI) PeerCount() hexutil.Uint {
	return hexutil.Uint(s.net.PeerCount())
}

// Version returns the current matrix protocol version.
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}
