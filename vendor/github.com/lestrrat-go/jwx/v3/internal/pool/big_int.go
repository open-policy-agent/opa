package pool

import (
	"math/big"
)

var bigIntPool = New(allocBigInt, destroyBigInt)

func BigInt() *Pool[*big.Int] {
	return bigIntPool
}

func allocBigInt() interface{} {
	return &big.Int{}
}

func destroyBigInt(i *big.Int) {
	i.SetInt64(0)
}
