package types

import (
	"io"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/rlp"
)

// if RPC
//Block: Block header uncles Currencies.Transactions
//Tx and TxReceipts : []Currencies.Transactions,[]Currencies.Receipts
//Save Raw Body []Currencies.Transactions
//Save Receipts []Currencies.Receipts
// Block represents an entire block in the Matrix blockchain.
type OldBlock struct {
	header     *OldHeader
	uncles     []*OldHeader
	currencies []CurrencyBlock
	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	td *big.Int

	// These fields are used by package man to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// [deprecated by man/63]
// StorageBlock defines the RLP encoding of a Block stored in the
// state database. The StorageBlock encoding contains fields that
// would otherwise need to be recomputed.
type StorageOldBlock OldBlock

// "external" block encoding. used for man protocol, etc.
type extoldblock struct {
	Header     *OldHeader
	Currencies []CurrencyBlock //BodyTransactions//[]SelfTransaction
	Uncles     []*OldHeader
}

// [deprecated by man/63]
// "storage" block encoding. used for database.
type storageoldblock struct {
	Header     *OldHeader
	Currencies []CurrencyBlock //[]SelfTransaction
	Uncles     []*OldHeader
	TD         *big.Int
}

// DecodeRLP decodes the Matrix
func (b *OldBlock) DecodeRLP(s *rlp.Stream) error {
	var eb extoldblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.currencies = eb.Header, eb.Uncles, eb.Currencies

	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Matrix RLP block format.
func (b *OldBlock) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extoldblock{
		Header:     b.header,
		Currencies: b.currencies,
		Uncles:     b.uncles,
	})
}

// [deprecated by man/63]
func (b *StorageOldBlock) DecodeRLP(s *rlp.Stream) error {
	var sb storageoldblock
	if err := s.Decode(&sb); err != nil {
		return err
	}
	b.header, b.uncles, b.currencies, b.td = sb.Header, sb.Uncles, sb.Currencies, sb.TD
	return nil
}

//old block不会单独使用，浅拷贝问题可忽略
func (ob *OldBlock) TransferBlock() *Block {
	var b Block
	b.header = ob.header.TransferHeader()
	b.uncles = make([]*Header, 0)
	for _, v := range ob.uncles {
		b.uncles = append(b.uncles, v.TransferHeader())
	}
	b.currencies = ob.currencies
	b.hash = ob.hash
	b.td = ob.td
	b.ReceivedAt = ob.ReceivedAt
	b.ReceivedFrom = ob.ReceivedFrom
	return &b
}
