/*
 * Copyright 2018 The ThunderDB Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the “License”);
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an “AS IS” BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package blockproducer

import (
	"math/big"

	"time"

	"testing"

	"reflect"

	"os"

	log "github.com/sirupsen/logrus"
	"github.com/thunderdb/ThunderDB/crypto/asymmetric"
	"github.com/thunderdb/ThunderDB/crypto/hash"
	"github.com/thunderdb/ThunderDB/proto"
	"github.com/thunderdb/ThunderDB/types"
)

var (
	voidTxSlice []*Tx
	txSlice     []*Tx
	header      SignedHeader
	header2     SignedHeader
	block       Block
	voidBlock   Block
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	voidTxSlice = []*Tx{}
	txSlice = make([]*Tx, 10)

	var i int64
	for i = 0; i < 10; i++ {
		_, pub, err := asymmetric.GenSecp256k1KeyPair()

		if err != nil {
			return
		}
		h := hash.THashH([]byte{byte(i)})
		txSlice[i] = &Tx{
			TxHash: h,
			TxData: TxData{
				AccountNonce: uint64(i),
				Recipient: &types.AccountAddress{
					AccountAddress: hash.THashH([]byte{byte(i * i)}).String(),
				},
				Amount:  big.NewInt(int64(i)),
				Payload: hash.THashB([]byte{byte(i / 2)}),
				Signature: &asymmetric.Signature{
					R: big.NewInt(1238 * i),
					S: big.NewInt(890321 / (i + 1)),
				},
				PublicKey: pub,
			},
		}
	}

	_, pub, err := asymmetric.GenSecp256k1KeyPair()
	if err != nil {
		return
	}

	header = SignedHeader{
		BlockHash: hash.THashH([]byte{1, 2, 3}),
		PublicKey: pub,
		Signature: &asymmetric.Signature{
			R: big.NewInt(8391),
			S: big.NewInt(2371),
		},
	}
	header.Version = 2
	header.Producer = proto.AccountAddress(hash.THashH([]byte{9, 1, 4, 2, 1, 10}).String())
	header.Root = hash.THashH([]byte{4, 2, 1, 10})
	header.Parent = hash.THashH([]byte{1, 9, 2, 22})
	header.MerkleRoot = hash.THashH([]byte{9, 2, 1, 11})
	header.Timestamp = time.Now().UTC()

	header2 = SignedHeader{
		BlockHash: hash.THashH([]byte{1, 2, 3}),
		PublicKey: pub,
		Signature: &asymmetric.Signature{
			R: big.NewInt(8391),
			S: big.NewInt(2371),
		},
	}
	header2.Version = 2
	header2.Producer = proto.AccountAddress(hash.THashH([]byte{1, 4, 2, 1, 10}).String())
	header2.Root = hash.THashH([]byte{4, 2, 1})
	header2.Parent = hash.THashH([]byte{1, 9, 22})
	header2.MerkleRoot = hash.THashH([]byte{9, 1, 11})
	header2.Timestamp = time.Now().UTC()

	voidBlock = Block{
		Header: &header,
		Tx:     voidTxSlice,
	}
	block = Block{
		Header: &header,
		Tx:     txSlice,
	}
}

func TestTx(t *testing.T) {
	for _, tx := range txSlice {
		_, err := tx.TxData.marshal()
		if err != nil {
			t.Errorf("cannot serialized TxData: %v", tx.TxData)
		}

		serializedTx, err := tx.marshal()
		if err != nil {
			t.Errorf("cannot serialized Tx: %v", tx)
		}
		deserializedTx := Tx{}
		err = deserializedTx.unmarshal(serializedTx)
		if err != nil {
			t.Errorf("cannot deserialized Tx buff: %v", serializedTx)
		}
		if !deserializedTx.TxHash.IsEqual(&tx.TxHash) {
			t.Errorf("deserialized tx hash is not equal to tx hash: \ntx: %v\ndeserializedTx: %v", tx, deserializedTx)
		}
		if !deserializedTx.TxData.Signature.IsEqual(tx.TxData.Signature) {
			t.Errorf("deserialized tx sign is not equal to tx sign: \ntx: %v\ndeserializedTx: %v", tx, deserializedTx)
		}
	}
}

func TestSignedHeader(t *testing.T) {
	serializedSignedHeader, err := header.marshal()
	if err != nil {
		t.Errorf("cannot serialized signedHeader: %v", header)
	}

	deserializedSignedHeader := SignedHeader{}
	err = deserializedSignedHeader.unmarshal(serializedSignedHeader)
	if err != nil {
		t.Errorf("cannot deserialized signedHeader: %v", serializedSignedHeader)
	}
	if !deserializedSignedHeader.Signature.IsEqual(header.Signature) {
		t.Errorf("deserialized header sign is not equal to header sign: \nheader: %v\ndeserializedBlock: %v",
			header, deserializedSignedHeader)
	}
	if !deserializedSignedHeader.BlockHash.IsEqual(&header.BlockHash) {
		t.Errorf("deserialized header block hash is not equal to header block hash: \nheader: %v\ndeserializedBlock: %v",
			header, deserializedSignedHeader)
	}
}

func TestBlock(t *testing.T) {
	blocks := make([]Block, 2)
	blocks[0] = voidBlock
	blocks[1] = block
	for i := 0; i < 2; i++ {
		serializedBlock, err := blocks[i].marshal()
		if err != nil {
			t.Errorf("cannot serialized block: %v", blocks[i])
		}

		deserializedBlock := Block{}
		err = deserializedBlock.unmarshal(serializedBlock)
		if err != nil {
			t.Errorf("cannot deserialized: %v", serializedBlock)
		}
		if !reflect.DeepEqual(deserializedBlock.Header.Header, blocks[i].Header.Header) {
			t.Errorf("deserialized block tx is not equal to block tx: \nblock: %v\ndeserializedBlock: %v",
				blocks[i].Header.Header.Producer, deserializedBlock.Header.Header.Producer)
		}
		for i := 0; i < len(blocks[i].Tx); i++ {
			if !deserializedBlock.Tx[i].TxHash.IsEqual(&blocks[i].Tx[i].TxHash) {
				t.Errorf("deserialized block tx #%d hash is not equal to block tx #%d hash: "+
					"\nblock: %v\ndeserializedBlock: %v",
					i, i, blocks[i].Tx[i], deserializedBlock.Tx[i])
			}
		}
		if !deserializedBlock.Header.Signature.IsEqual(blocks[i].Header.Signature) {
			t.Errorf("deserialized block sign is not equal to block sign: \nblock: %v\ndeserializedBlock: %v",
				blocks[i], deserializedBlock)
		}

	}
}
