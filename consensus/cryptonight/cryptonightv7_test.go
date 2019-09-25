// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php
// Copyright 2017-2018 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package cryptonight

import "fmt"
import "testing"
import "encoding/hex"

// test cases from original implementation
func Test_Cryptonightv7_Hash(t *testing.T) {

	tests := []struct {
		data     string
		expected string
	}{
		{
			data:     "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			expected: "b5a7f63abb94d07d1a6445c36c07c7e8327fe61b1647e391b4c7edae5de57a3d",
		},
		{
			data:     "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			expected: "80563c40ed46575a9e44820d93ee095e2851aa22483fd67837118c6cd951ba61",
		},
		{
			data:     "8519e039172b0d70e5ca7b3383d6b3167315a422747b73f019cf9528f0fde341fd0f2a63030ba6450525cf6de31837669af6f1df8131faf50aaab8d3a7405589",
			expected: "5bb40c5880cef2f739bdb6aaaf16161eaae55530e7b10d7ea996b751a299e949",
		},
		{
			data:     "37a636d7dafdf259b7287eddca2f58099e98619d2f99bdb8969d7b14498102cc065201c8be90bd777323f449848b215d2977c92c4c1c2da36ab46b2e389689ed97c18fec08cd3b03235c5e4c62a37ad88c7b67932495a71090e85dd4020a9300",
			expected: "613e638505ba1fd05f428d5c9f8e08f8165614342dac419adc6a47dce257eb3e",
		},
		{
			data:     "38274c97c45a172cfc97679870422e3a1ab0784960c60514d816271415c306ee3a3ed1a77e31f6a885c3cb",
			expected: "ed082e49dbd5bbe34a3726a0d1dad981146062b39d36d62c71eb1ed8ab49459b",
		},
	}

	for _, test := range tests {
		data, err := hex.DecodeString(test.data)
		if err != nil {
			t.Fatalf("Could NOT decode test")
		}

		actual := SlowHashv7(data)
		//t.Logf("cryptonightv7: want: %s, got: %x", test.expected, actual)
		if fmt.Sprintf("%x", actual) != test.expected {
			t.Fatalf("cryptonightv7: want: %s, got: %x", test.expected, actual)
			continue
		}

	}

}
