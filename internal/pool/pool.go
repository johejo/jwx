package pool

import (
	"bytes"
	"math/big"
	"sync"
)

var bytesBufferPool = sync.Pool{
	New: allocBytesBuffer,
}

func allocBytesBuffer() interface{} {
	return &bytes.Buffer{}
}

func GetBytesBuffer() *bytes.Buffer {
	return bytesBufferPool.Get().(*bytes.Buffer)
}

func ReleaseBytesBuffer(b *bytes.Buffer) {
	b.Reset()
	bytesBufferPool.Put(b)
}

var bigIntPool = sync.Pool{
	New: allocBigInt,
}

func allocBigInt() interface{} {
	return &big.Int{}
}

func GetBigInt() *big.Int {
	return bigIntPool.Get().(*big.Int)
}

func ReleaseBigInt(i *big.Int) {
	bigIntPool.Put(i.SetInt64(0))
}

var scratchMapPool = sync.Pool {
	New: allocScratchMap,
}

func allocScratchMap() interface{} {
	return make(map[string]interface{})
}

func GetScratchMap() map[string]interface{} {
	return scratchMapPool.Get().(map[string]interface{})
}

func ReleaseScratchMap(m map[string]interface{}) {
	// Asyncrhonously delete
	go func() {
		for k := range m {
			delete(m, k)
		}
		scratchMapPool.Put(m)
	}()
}
