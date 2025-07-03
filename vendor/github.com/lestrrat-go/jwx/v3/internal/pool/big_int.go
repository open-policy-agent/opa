package pool

import "math/big"

var bigIntPool = New[*big.Int](allocBigInt, freeBigInt)

func allocBigInt() *big.Int {
	return &big.Int{}
}

func freeBigInt(b *big.Int) *big.Int {
	b.SetInt64(0) // Reset the value to zero
	return b
}

// BigInt returns a pool of *big.Int instances.
func BigInt() *Pool[*big.Int] {
	return bigIntPool
}
