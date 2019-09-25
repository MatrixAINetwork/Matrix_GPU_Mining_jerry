// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package types

import (
	"errors"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/params/manversion"
	"github.com/MatrixAINetwork/go-matrix/rlp"
	"math/big"
	"testing"
)

func TestHeaderEncoding(t *testing.T) {
	log.InitLog(3)

	header1 := &Header{
		Number:  big.NewInt(999),
		Version: []byte(manversion.VersionAIMine),
		AIHash:  common.HexToHash("0xffffffffffffffff"),
	}
	header2 := &Header{
		Number:   big.NewInt(88),
		Version:  []byte(manversion.VersionDelta),
		AIHash:   common.HexToHash("0xeeeeeeeeeeeeeeee"),
		VrfValue: []byte("test data 1111"),
	}
	header3 := &Header{
		Number:  big.NewInt(777),
		Version: []byte(manversion.VersionAIMine),
		AIHash:  common.HexToHash("0xdddddddddddddddd"),
	}

	headers := make([]*Header, 0)
	headers = append(headers, header1)
	headers = append(headers, header2)
	headers = append(headers, header3)

	data, err := rlp.EncodeToBytes(headers)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	log.Info("rlp data", "data", len(data))

	kind, listContent, rest, err := rlp.Split(data)
	log.Info("list split", "kind", kind, "list_content", len(listContent), "rest", len(rest), "err", err)

	if err != nil && kind != rlp.List && len(rest) != 0 {
		log.Error("数据类型不是list")
		t.Fatal(err)
	}

	rest = listContent
	var result []*Header
	for len(rest) != 0 {
		one, restData, err := decodeOneHeader(rest)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, one)
		rest = restData
	}
	log.Info("成功", "result", result)
}

func decodeOneHeader(data []byte) (header *Header, rest []byte, err error) {
	var kind rlp.Kind
	kind, _, rest, err = rlp.Split(data)
	log.Info("content split", "kind", kind, "rest", len(rest), "err", err)
	if err != nil {
		return nil, nil, err
	}
	if kind != rlp.List {
		return nil, nil, errors.New("header data kind err")
	}

	var nh Header
	err = rlp.DecodeBytes(data[:len(data)-len(rest)], &nh)
	if err == nil {
		log.Info("解码成功", "new header", nh.Number)
		return &nh, rest, err
	} else {
		var oh OldHeader
		err = rlp.DecodeBytes(data[:len(data)-len(rest)], &oh)
		if err == nil {
			log.Info("解码成功", "old header", nh.Number)
			return oh.TransferHeader(), rest, err
		} else {
			return nil, nil, err
		}
	}
}
