// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package syn

import (
	"crypto/md5"
	"github.com/pkg/errors"
	"hash"
	"io"
	"os"
)

// Signatures reads data blocks from reader and pipes out block signatures on the
// returning channel, closing it when done reading or when the context is cancelled.
// This function does not block and returns immediately. The caller must make sure the concrete
// reader instance is not nil or this function will panic.
func Signatures(r io.Reader, h hash.Hash, consumer func(*BlockSignature) error) error {
	var index uint64

	bfp := bufferPool.Get().(*[]byte)
	buffer := *bfp
	defer bufferPool.Put(bfp)

	if r == nil {
		return errors.New("reader required")
	}

	if h == nil {
		h = md5.New()
	}
	for {

		n, err := r.Read(buffer)
		if err == io.EOF {
			break
		}

		if err != nil {
			return errors.Wrapf(err, "failed reading block")
		}

		block := buffer[:n]
		h.Reset()
		h.Write(block)
		strong := h.Sum(nil)
		_, _, rhash := rollingHash(block)

		sig := BlockSignature{
			Index:  index,
			Weak:   rhash,
			Strong: strong,
		}

		if err := consumer(&sig); err != nil {
			return err
		}
		index++
	}

	return nil
}

func GetTotalSignatures(r io.Reader, h hash.Hash) ([]*BlockSignature, error) {
	sigs := make([]*BlockSignature, 0)
	err := Signatures(r, h, func(signature *BlockSignature) error {
		sigs = append(sigs, signature)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return sigs, nil
}

// Apply reconstructs a file given a set of operations. The caller must close the ops channel or the context when done or there will be a deadlock.
func Apply(dst io.Writer, cache io.ReaderAt, supplier func() (*BlockOperation, error)) error {
	bfp := bufferPool.Get().(*[]byte)
	buffer := *bfp
	defer bufferPool.Put(bfp)

	for {
		o, err := supplier()
		// error termination
		if err != nil {
			return err
		}
		// normal termination
		if o == nil {
			break
		}

		var block []byte

		if len(o.Data) > 0 {
			block = o.Data
		} else {
			if f, ok := cache.(*os.File); ok && f == nil {
				return errors.New("index operation, but cached file was not found")
			}

			index := int64(o.Index)
			n, err := cache.ReadAt(buffer, index*DefaultBlockSize)
			if err != nil && err != io.EOF {
				return errors.Wrapf(err, "failed reading cached block")
			}

			block = buffer[:n]
		}

		_, err = dst.Write(block)
		if err != nil {
			return errors.Wrapf(err, "failed writing block to destination")
		}
	}
	return nil
}
