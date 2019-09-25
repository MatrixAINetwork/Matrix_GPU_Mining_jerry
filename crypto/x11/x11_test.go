// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package x11

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFunc(t *testing.T) {
	input := common.Hex2Bytes("0000002002aacd6033b9ba23494220d2af3b0ccf8b02b06a533b08bd0e00000000000000be172e11d0d4f444c1c1d5678f665e14668872cdb618f256af14e9674d703a8e3f8c355d6b451a1973f802ec")
	t.Log(input)
	hash := Hash(input)
	t.Log(hash)

	binary.Read(bytes.NewReader(hash), binary.BigEndian, hash)

	hexStr := common.BytesToHash(hash).Hex()
	t.Log(hexStr)
}

func TestHash(t *testing.T) {
	for i := range tsInfo {
		ln := len(tsInfo[i].out)
		dest := make([]byte, ln)

		out := Hash(tsInfo[i].in[:])
		if ln != hex.Encode(dest, out[:]) {
			t.Errorf("%s: invalid length", tsInfo[i])
		}
		if !bytes.Equal(dest[:], tsInfo[i].out[:]) {
			t.Errorf("%s: invalid hash", tsInfo[i].id)
		}

		hash := common.BytesToHash(out)
		t.Logf("%d hash: %s", i, hash.Hex())

	}
}

////////////////

var tsInfo = []struct {
	id  string
	in  []byte
	out []byte
}{
	{
		"Empty",
		[]byte(""),
		[]byte("51b572209083576ea221c27e62b4e22063257571ccb6cc3dc3cd17eb67584eba"),
	},
	{
		"Dash",
		[]byte("DASH"),
		[]byte("fe809ebca8753d907f6ad32cdcf8e5c4e090d7bece5df35b2147e10b88c12d26"),
	},
	{
		"Fox",
		[]byte("The quick brown fox jumps over the lazy dog"),
		[]byte("534536a4e4f16b32447f02f77200449dc2f23b532e3d9878fe111c9de666bc5c"),
	},
}

func TestHashaaaa(t *testing.T) {
	input := common.Hex2Bytes("0000002002aacd6033b9ba23494220d2af3b0ccf8b02b06a533b08bd0e00000000000000be172e11d0d4f444c1c1d5678f665e14668872cdb618f256af14e9674d703a8e3f8c355d6b451a1973f802ec")
	out := Hash(input[:])
	fmt.Println(common.ToHex(out))
}
func TestHashReverse(t *testing.T) {
	a := "08316e1008d9b29c7ffe136276179bd65f84818873639426e1aa4b7a00000000"
	//aa, _ := new(big.Int).SetString(a, 16)
	//fmt.Println(aa)
	ret := Reverse(common.FromHex(a))
	aaret := strings.TrimPrefix(ret, "0x")
	aa, _ := new(big.Int).SetString(aaret, 16)
	fmt.Println(aa)
	fmt.Println(ret)
	fmt.Println(aaret)
	b := "0000000000000000000000000000000000000000000000000000000001000000"
	//bb, _ := new(big.Int).SetString(b, 16)
	//fmt.Println(bb)
	ret = Reverse(common.FromHex(b))
	aaret = strings.TrimPrefix(ret, "0x")
	bb, _ := new(big.Int).SetString(aaret, 16)
	fmt.Println(bb)
	fmt.Println(ret)
	fmt.Println(aaret)
	if aa.Cmp(bb) > 0 {
		fmt.Println("aaaaaaaaaaaaaaaaa")
	}
	if aa.Cmp(bb) < 0 {
		fmt.Println("bbbbbbbbbbbbb")
	}
}
func Reverse(s []byte) string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return common.ToHex(s)
}
func TestJumpDestAnalysisaaaa(t *testing.T) {
	strAddr := "MAN.tkzuVC83cth5Z6RBGApDnaWVXGvo.abc"
	lastindex := strings.LastIndex(strAddr, ".")
	str := strAddr[:lastindex+1]
	fmt.Println(str)
}
func hashString2LittleEndian(hash string) string {
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
func TestJumpDestAnalysisbbbb(t *testing.T) {
	t1 := time.Now()
	t2 := time.Since(t1)
	fmt.Println("calling AIDiggingByMiner time ", t2)
	tmpnonce, _ := conversType("fd95251415168917861581568561816489164164868486456580")
	fmt.Println(tmpnonce.Uint64())
	tmpnonce, _ = conversType("")
	fmt.Println(tmpnonce.Uint64())
	strnonce, _ := strconv.ParseUint(strings.Replace("fd95251410", "0x", "", -1), 16, 32)
	nonce := uint32(strnonce)
	fmt.Println(nonce)
	nonce64 := uint64(strnonce)
	fmt.Println(nonce64)
	bigNonce, _ := new(big.Int).SetString("fd95251410", 16)
	fmt.Println(bigNonce)
	strnonce, _ = strconv.ParseUint(strings.Replace("fd9525", "0x", "", -1), 16, 32)
	nonce = uint32(strnonce)
	fmt.Println(nonce)
	fmt.Println(uint64(strnonce))
	bigNonce, _ = new(big.Int).SetString("fd9525", 16)
	fmt.Println(bigNonce)

	aiHash := "b83d311f2c6d5972cada66386a13949d9b31de8112f0c64575fa2da5c06ee04f"
	bytelen := common.Hex2Bytes(aiHash)
	leng := len(bytelen)
	fmt.Println(leng)
	fmt.Println(len(aiHash))
	aiHash = aiHash[2:]
	coinbase := common.HexToAddress("0xc24c56af638a788b76d7e9c058f29680b323344e")
	seed := big.NewInt(0).Add(coinbase.Big(), coinbase.Big()).Int64()
	fmt.Println(strconv.FormatInt(seed, 16))
	fmt.Println(strconv.FormatInt(seed, 10))
	str1 := strconv.FormatInt(seed, 10)
	int1, _ := strconv.Atoi(str1)
	fmt.Println(int64(int1))
	seedbig, _ := new(big.Int).SetString("6e0c049cf63e72", 16)
	fmt.Println(seedbig.Int64())
	var params []string
	var paramsTmp []string
	params = append(params, "1")
	params = append(params, "2")
	params = append(params, "3")
	params = append(params, "4")
	params = append(params, "5")
	paramsTmp = append(paramsTmp, params[:2]...)
	paramsTmp = append(paramsTmp, "aaaaaaaaaaaaaaaaaaa")
	paramsTmp = append(paramsTmp, params[2:]...)

	ret := strings.Compare("aa", "aa")
	fmt.Println(ret)
	ret = strings.Compare("aa", "aaa")
	fmt.Println(ret)
	ret = strings.Compare("aaaa", "aaa")
	fmt.Println(ret)

	//blocknonce := types.BlockNonce{}
	//strnonce := common.FromHex("1a000000")
	//for i := 0; i < len(strnonce); i++ {
	//	blocknonce[i] = strnonce[i] //- '0'
	//}
	//var nonce []byte
	//for i := 0; i < len(blocknonce); i++ {
	//	nonce = append(nonce, blocknonce[i]) //- '0'
	//}
	//fmt.Println(nonce)
	//Reverse(nonce)
	//databufio := bytes.NewBuffer([]byte{})
	//binary.Write(databufio, binary.LittleEndian, nonce)
	//fmt.Println(nonce)
	//a := new(big.Int).SetBytes(nonce)
	//fmt.Println(uint32(a.Uint64()))
	//fmt.Println(uint32ToBytes(uint32(a.Uint64())))
	//fmt.Println(databufio.Bytes())

}
func conversType(str string) (types.BlockNonce, error) {
	nonce := types.BlockNonce{}
	//strnonce := strings.TrimPrefix(str, "0x")
	strnonce := common.FromHex(str)
	//if len(strnonce) == 8 {
	for i := 0; i < len(strnonce); i++ {
		nonce[i] = strnonce[i] //- '0'
	}
	//} else {
	//	return types.BlockNonce{}, errors.New("submitWork,recv nonce len less 8")
	//}
	return nonce, nil
}
func uint32ToBytes(num uint32) []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, num)
	return data
}
func Reversebyte(s []byte) []byte {
	var aa []byte
	for i := 8; i > 0; i-- {
		aa = append(aa, s[i-2:i]...)
		i--
	}
	return aa
}
