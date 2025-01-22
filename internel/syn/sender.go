// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package syn

import (
	"bytes"
	"crypto/md5"
	"hash"
	"io"

	"github.com/pkg/errors"
)

// LookUpTable reads up blocks signatures and builds a lookup table for the client to search from when trying to decide
// whether to send or not a block of data.
func LookUpTable(supplier func() (*BlockSignature, error)) (map[uint32][]BlockSignature, error) {
	table := make(map[uint32][]BlockSignature)
	for {
		c, err := supplier()
		if err != nil {
			return nil, err
		}
		if c == nil {
			break
		}
		table[c.Weak] = append(table[c.Weak], *c)
	}

	return table, nil
}

// Sync sends tokens or literal bytes to the caller in order to efficiently re-construct a remote file. Whether to send
// tokens or literals is determined by the remote checksums provided by the caller.
// This function does not block and returns immediately. Also, the remote blocks map is accessed without a mutex,
// so this function is expected to be called once the remote blocks map is fully populated.
//
// The caller must make sure the concrete reader instance is not nil or this function will panic.
func Sync(r io.ReaderAt, shash hash.Hash, remote map[uint32][]BlockSignature, consumer func(BlockOperation) error) error {
	if r == nil {
		return errors.New("reader required")
	}

	if shash == nil {
		shash = md5.New()
	}

	var (
		r1, r2, rhash, old uint32
		offset             int64
		rolling, match     bool
	)

	delta := make([]byte, 0)

	for {

		bfp := bufferPool.Get().(*[]byte)
		buffer := *bfp

		n, err := r.ReadAt(buffer, offset)
		if err != nil && err != io.EOF {
			bufferPool.Put(bfp)
			// return since data corruption in the server is possible and a re-sync is required.
			return errors.Wrapf(err, "failed reading data block")
		}

		block := buffer[:n]

		// If there are no block signatures from remote server, send all data blocks
		if len(remote) == 0 {
			if n > 0 {
				op := BlockOperation{Data: block}
				if err := consumer(op); err != nil {
					return err
				}
				offset += int64(n)
			}

			if err == io.EOF {
				bufferPool.Put(bfp)
				return nil
			}
			continue
		}

		if rolling {
			iv := uint32(block[n-1])
			r1, r2, rhash = rollingHash2(uint32(n), r1, r2, old, iv)
		} else {
			r1, r2, rhash = rollingHash(block)
		}

		if bs, ok := remote[rhash]; ok {
			shash.Reset()
			shash.Write(block)
			s := shash.Sum(nil)

			for _, b := range bs {
				if !bytes.Equal(s, b.Strong) {
					continue
				}

				match = true

				// We need to send deltas before sending an index token.
				if len(delta) > 0 {
					if err := send(bytes.NewReader(delta), consumer); err != nil {
						return err
					}
					delta = make([]byte, 0)
				}

				// instructs the server to copy block data at offset b.Index
				// from its own copy of the file.
				op := BlockOperation{Index: b.Index}
				if err := consumer(op); err != nil {
					return err
				}
				break
			}
		}

		if match {
			if err == io.EOF {
				bufferPool.Put(bfp)
				break
			}

			rolling, match = false, false
			old, rhash, r1, r2 = 0, 0, 0, 0
			offset += int64(n)
		} else {
			if err == io.EOF {
				// If EOF is reached and not match data found, we add trailing data
				// to delta array.
				delta = append(delta, block...)
				if len(delta) > 0 {
					if err := send(bytes.NewReader(delta), consumer); err != nil {
						return err
					}
				}
				bufferPool.Put(bfp)
				break
			}
			rolling = true
			old = uint32(block[0])
			delta = append(delta, block[0])
			offset++
		}

		// Returning this buffer to the pool here gives us 5x more speed
		bufferPool.Put(bfp)
	}

	return nil
}

// send sends all deltas over the channel. Any error is reported back using the
// same channel.
func send(r io.Reader, consumer func(BlockOperation) error) error {
	for {

		bfp := bufferPool.Get().(*[]byte)
		buffer := *bfp
		defer bufferPool.Put(bfp)

		n, err := r.Read(buffer)
		if err != nil && err != io.EOF {
			return errors.Wrapf(err, "failed reading data block")
		}

		// If we don't guard against 0 bytes reads, an operation with index 0 will be sent
		// and the server will duplicate block 0 at the end of the reconstructed file.
		if n > 0 {
			block := buffer[:n]
			op := BlockOperation{Data: block}
			if err := consumer(op); err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
	}
	return nil
}
