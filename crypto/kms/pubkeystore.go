/*
 * Copyright 2018 The ThunderDB Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kms

import (
	"errors"

	"sync"

	"bytes"

	log "github.com/sirupsen/logrus"

	"github.com/coreos/bbolt"
	"github.com/thunderdb/ThunderDB/crypto/asymmetric"
	"github.com/thunderdb/ThunderDB/crypto/hash"
	mine "github.com/thunderdb/ThunderDB/pow/cpuminer"
	"github.com/thunderdb/ThunderDB/proto"
	"github.com/ugorji/go/codec"
)

// PublicKeyStore holds db and bucket name
type PublicKeyStore struct {
	db     *bolt.DB
	bucket []byte
}

const (
	// kmsBucketName is the boltdb bucket name
	kmsBucketName = "kms"
)

var (
	// pks holds the singleton instance
	pks *PublicKeyStore
	// PksOnce for easy test we make PksOnce exported
	PksOnce sync.Once
	// Unittest is a test flag
	Unittest bool
)

var (
	// BPPublicKeyStr is the public key of Block Producer
	BPPublicKeyStr = "02c1db96f2ba7e1cb4e9822d12de0f63f" +
		"b666feb828c7f509e81fab9bd7a34039c"
	// BPNodeID is the node id of Block Producer
	// 	{{789554103 0 0 8070450536379825883}    43 000000000013fd4b3180dd424d5a895bc57b798e5315087b7198c926d8893f98}
	BPNodeID = "000000000013fd4b3180dd424d5a895bc57b798e5315087b7198c926d8893f98"
	// BPRawNodeID hold the binary hash version of BPNodeID, will be initialized
	// at init()
	BPRawNodeID proto.RawNodeID
	// BPNonce is the nonce, SEE: cmd/idminer for more
	BPNonce = mine.Uint256{
		789554103,
		0,
		0,
		8070450536379825883,
	}
	// BPPublicKey point to BlockProducer public key
	BPPublicKey *asymmetric.PublicKey
)

func init() {
	err := hash.Decode(&BPRawNodeID.Hash, BPNodeID)
	if err != nil {
		log.Fatalf("BPNodeID error: %s", err)
	}
}

var (
	// ErrBucketNotInitialized indicates bucket not initialized
	ErrBucketNotInitialized = errors.New("bucket not initialized")
	// ErrNilNode indicates input node is nil
	ErrNilNode = errors.New("nil node")
	// ErrKeyNotFound indicates key not found
	ErrKeyNotFound = errors.New("key not found")
	// ErrNotValidNodeID indicates that is not valid node id
	ErrNotValidNodeID = errors.New("not valid node id")
	// ErrNodeIDKeyNonceNotMatch indicates node id, key, nonce not match
	ErrNodeIDKeyNonceNotMatch = errors.New("nodeID, key, nonce not match")
)

// InitPublicKeyStore opens a db file, if not exist, creates it.
// and creates a bucket if not exist
func InitPublicKeyStore(dbPath string, initNode *proto.Node) (err error) {
	var bdb *bolt.DB
	bdb, err = bolt.Open(dbPath, 0600, nil)
	if err != nil {
		log.Errorf("InitPublicKeyStore failed: %s", err)
		return
	}

	name := []byte(kmsBucketName)
	err = (*bolt.DB)(bdb).Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(name); err != nil {
			log.Errorf("could not create bucket: %s", err)
			return err
		}
		return nil // return from Update func
	})
	if err != nil {
		log.Errorf("InitPublicKeyStore failed: %s", err)
		return
	}

	// pks is the singleton instance
	pks = &PublicKeyStore{
		db:     bdb,
		bucket: name,
	}

	if initNode != nil {
		err = setNode(initNode)
	}

	return
}

// GetPublicKey gets a PublicKey of given id
// Returns an error if the id was not found
func GetPublicKey(id proto.NodeID) (publicKey *asymmetric.PublicKey, err error) {
	node, err := GetNodeInfo(id)
	if err == nil {
		publicKey = node.PublicKey
	}
	return
}

// GetNodeInfo gets node info of given id
// Returns an error if the id was not found
func GetNodeInfo(id proto.NodeID) (nodeInfo *proto.Node, err error) {
	err = (*bolt.DB)(pks.db).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(pks.bucket)
		if bucket == nil {
			return ErrBucketNotInitialized
		}
		byteVal := bucket.Get([]byte(id))
		if byteVal == nil {
			return ErrKeyNotFound
		}
		log.Debugf("get node: %#v", byteVal)
		reader := bytes.NewReader(byteVal)
		mh := &codec.MsgpackHandle{}
		dec := codec.NewDecoder(reader, mh)
		nodeInfo = proto.NewNode()
		err = dec.Decode(nodeInfo)
		return err // return from View func
	})
	if err != nil {
		log.Errorf("get node info failed: %s", err)
	}
	return
}

// GetAllNodeID get all node ids exist in store
func GetAllNodeID() (nodeIDs []proto.NodeID, err error) {
	err = (*bolt.DB)(pks.db).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(pks.bucket)
		if bucket == nil {
			return ErrBucketNotInitialized
		}
		err := bucket.ForEach(func(k, v []byte) error {
			nodeIDs = append(nodeIDs, proto.NodeID(k))
			return nil
		})

		return err // return from View func
	})
	if err != nil {
		log.Errorf("get all node id failed: %s", err)
	}
	return

}

// SetPublicKey verifies nonce and set Public Key
func SetPublicKey(id proto.NodeID, nonce mine.Uint256, publicKey *asymmetric.PublicKey) (err error) {
	nodeInfo := &proto.Node{
		ID:        id,
		Addr:      "",
		PublicKey: publicKey,
		Nonce:     nonce,
	}
	return SetNode(nodeInfo)
}

// SetNode verifies nonce and sets {proto.Node.ID: proto.Node}
func SetNode(nodeInfo *proto.Node) (err error) {
	if nodeInfo == nil {
		return ErrNilNode
	}
	if !Unittest {
		if nodeInfo.PublicKey == nil {
			return ErrNilNode
		}
		keyHash := mine.HashBlock(nodeInfo.PublicKey.Serialize(), nodeInfo.Nonce)
		id := nodeInfo.ID
		idHash, err := hash.NewHashFromStr(string(id))
		if err != nil {
			return ErrNotValidNodeID
		}
		if !keyHash.IsEqual(idHash) {
			return ErrNodeIDKeyNonceNotMatch
		}
	}

	return setNode(nodeInfo)
}

// setNode sets id and its publicKey
func setNode(nodeInfo *proto.Node) (err error) {
	nodeBuf := new(bytes.Buffer)
	mh := &codec.MsgpackHandle{}
	enc := codec.NewEncoder(nodeBuf, mh)
	err = enc.Encode(*nodeInfo)
	if err != nil {
		log.Errorf("marshal node info failed: %s", err)
		return
	}
	log.Debugf("set node: %#v", nodeBuf.Bytes())

	err = (*bolt.DB)(pks.db).Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(pks.bucket)
		if bucket == nil {
			return ErrBucketNotInitialized
		}
		return bucket.Put([]byte(nodeInfo.ID), nodeBuf.Bytes())
	})
	if err != nil {
		log.Errorf("get node info failed: %s", err)
	}

	return
}

// DelNode removes PublicKey to the id
func DelNode(id proto.NodeID) (err error) {
	err = (*bolt.DB)(pks.db).Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(pks.bucket)
		if bucket == nil {
			return ErrBucketNotInitialized
		}
		return bucket.Delete([]byte(id))
	})
	if err != nil {
		log.Errorf("del node failed: %s", err)
	}
	return
}

// removeBucket this bucket
func removeBucket() (err error) {
	err = (*bolt.DB)(pks.db).Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte(kmsBucketName))
	})
	if err != nil {
		log.Errorf("remove bucket failed: %s", err)
		return
	}
	// ks.bucket == nil means bucket not exist
	pks.bucket = nil
	return
}

// ResetBucket this bucket
func ResetBucket() error {
	// cause we are going to reset the bucket, the return of removeBucket
	// is not useful
	removeBucket()
	bucketName := []byte(kmsBucketName)
	err := (*bolt.DB)(pks.db).Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	pks.bucket = bucketName
	if err != nil {
		log.Errorf("reset bucket failed: %s", err)
	}

	return err
}
