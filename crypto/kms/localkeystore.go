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
	"sync"

	"runtime"

	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/thunderdb/ThunderDB/crypto/asymmetric"
	mine "github.com/thunderdb/ThunderDB/pow/cpuminer"
)

// LocalKeyStore is the type hold local private & public key
type LocalKeyStore struct {
	isSet     bool
	private   *asymmetric.PrivateKey
	public    *asymmetric.PublicKey
	nodeID    []byte
	nodeNonce *mine.Uint256
	sync.RWMutex
}

var (
	// localKey is global accessible local private & public key
	localKey     *LocalKeyStore
	localKeyOnce sync.Once
)

var (
	// ErrNilField indicates field is nil
	ErrNilField = errors.New("local field is nil")
)

// InitLocalKeyStore returns a new LocalKeyStore
func InitLocalKeyStore() {
	localKeyOnce.Do(func() {
		localKey = &LocalKeyStore{
			isSet:     false,
			private:   nil,
			public:    nil,
			nodeID:    nil,
			nodeNonce: nil,
		}
	})
}

// SetLocalKeyPair sets private and public key, this is a one time thing
func SetLocalKeyPair(private *asymmetric.PrivateKey, public *asymmetric.PublicKey) {
	localKey.Lock()
	defer localKey.Unlock()
	if localKey.isSet {
		return
	}
	localKey.isSet = true
	localKey.private = private
	localKey.public = public
}

// SetLocalNodeIDNonce sets private and public key, this is a one time thing
func SetLocalNodeIDNonce(rawNodeID []byte, nonce *mine.Uint256) {
	localKey.Lock()
	defer localKey.Unlock()
	localKey.nodeID = make([]byte, len(rawNodeID))
	copy(localKey.nodeID, rawNodeID)
	if nonce != nil {
		localKey.nodeNonce = new(mine.Uint256)
		*localKey.nodeNonce = *nonce
	}
}

// GetLocalNodeID gets current node ID copy in []byte
func GetLocalNodeID() (rawNodeID []byte, err error) {
	localKey.RLock()
	if localKey.nodeID != nil {
		rawNodeID = make([]byte, len(localKey.nodeID))
		copy(rawNodeID, localKey.nodeID)
	} else {
		err = ErrNilField
	}
	localKey.RUnlock()
	return
}

// GetLocalNonce gets current node nonce copy
func GetLocalNonce() (nonce *mine.Uint256, err error) {
	localKey.RLock()
	if localKey.nodeNonce != nil {
		nonce = new(mine.Uint256)
		*nonce = *localKey.nodeNonce
	} else {
		err = ErrNilField
	}
	localKey.RUnlock()
	return
}

// GetLocalPublicKey gets local public key, if not set yet returns nil
func GetLocalPublicKey() (public *asymmetric.PublicKey, err error) {
	localKey.RLock()
	public = localKey.public
	if public == nil {
		err = ErrNilField
	}
	localKey.RUnlock()
	return
}

// GetLocalPrivateKey gets local private key, if not set yet returns nil
//  all call to this func will be logged
func GetLocalPrivateKey() (private *asymmetric.PrivateKey, err error) {
	localKey.RLock()
	private = localKey.private
	if private == nil {
		err = ErrNilField
	}
	localKey.RUnlock()

	// log the call stack
	buf := make([]byte, 4096)
	count := runtime.Stack(buf, false)
	log.Infof("###getting private key from###\n%s\n###getting private  key end###\n", buf[:count])
	return
}
