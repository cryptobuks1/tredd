package tedd

import (
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/bobg/merkle"
	"github.com/chain/txvm/errors"
)

// Serve produces a stream of interleaved <clearhash><cipherchunk> pairs from the content in r.
// It writes the stream to w, encrypting the chunks by xoring with hashes derived from key.
// The return value is the Merkle root hash of the cipher chunks, each prepended with its chunk index.
// TODO: Cleartext chunks and their hashes can be precomputed and supplied as ChunkStores.
func Serve(w io.Writer, r io.Reader, key [32]byte) ([]byte, error) {
	var (
		cipherMT            = merkle.NewTree(sha256.New())
		hasher              = sha256.New()
		chunkWithPrefix     [ChunkSize + binary.MaxVarintLen64]byte
		clearHashWithPrefix [32 + binary.MaxVarintLen64]byte
	)

	for index := uint64(0); ; index++ {
		m := binary.PutUvarint(chunkWithPrefix[:], index)
		binary.PutUvarint(clearHashWithPrefix[:], index)

		n, err := io.ReadFull(r, chunkWithPrefix[m:m+ChunkSize])
		if err == io.EOF {
			// "The error is EOF only if no bytes were read."
			break
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, errors.Wrapf(err, "reading clear chunk %d", index)
		}

		merkle.LeafHash(hasher, clearHashWithPrefix[:m], chunkWithPrefix[:m+n])

		_, err = w.Write(clearHashWithPrefix[m : m+32])
		if err != nil {
			return nil, errors.Wrapf(err, "writing clear hash %d", index)
		}

		crypt(key, chunkWithPrefix[m:m+n], index) // n.b. overwrites the contents of chunk
		_, err = w.Write(chunkWithPrefix[m : m+n])
		if err != nil {
			return nil, errors.Wrapf(err, "writing cipher chunk %d", index)
		}
		cipherMT.Add(chunkWithPrefix[:m+n])
	}

	return cipherMT.Root(), nil
}
