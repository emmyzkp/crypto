/*
 * Copyright 2017 XLAB d.o.o.
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
 *
 */

package qoneway

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/emmyzkp/crypto/rsa"
)

// RSABased represents RSA-based q-one-way.
type RSABased struct {
	Group *rsa.Group
	// Q is a random number > Group.N.
	Q *big.Int
	// Homomorphism is q-one-way Homomorphism f: x -> x^Q mod N.
	// It is difficult to compute a preimage of y^i for i < Q, but easy for i = Q.
	// Computing preimage of y^Q for RSA-based q-one-way is trivial: it is y.
	Homomorphism func(*big.Int) *big.Int
	// HomomorphismInv can compute x such that Homomorphism(x) = y^Q, given y^Q.
	// Note: we assume that HomomorphismInv takes y as input, not y^Q.
	// In our case (RSA-based q-one-way), HomomorphismInv is trivial: identity.
	// For other QOneHomomorphisms it might be different.
	HomomorphismInv func(*big.Int) *big.Int
}

// NewRSABased generates a new instance of RSABased q-one-way.
// It takes bit length for instantiating the underlying rsa.Group.
func NewRSABased(bitLen int) (*RSABased, error) {
	rsa, err := rsa.NewGroup(bitLen)
	if err != nil {
		return nil, err
	}

	q, err := rand.Prime(rand.Reader, bitLen+1)
	if err != nil {
		return nil, err
	}

	if q.Cmp(rsa.N) < 1 {
		return nil, fmt.Errorf("Q must be > N")
	}

	rsa.E = q

	homomorphismInv := func(y *big.Int) *big.Int {
		return y
	}

	return &RSABased{
		Q:               q,
		Group:           rsa,
		Homomorphism:    rsa.Homomorphism,
		HomomorphismInv: homomorphismInv,
	}, nil
}

// Committer implements commitment scheme based on RSA based q-one-way Group Homomorphism
// (scheme proposed by Cramer and Damgard). Commitment schemes based on q-one-way Homomorphism
// have some nice properties - it can be proved in zero knowledge that a commitment
// contains 0 or 1 (see ProveBitCommitment) and it can be proved for A, B, C that C
// is commitment for a * b where A is commitment to a and B commitment to B.
type Committer struct {
	*RSABased
	Y              *big.Int
	committedValue *big.Int
	r              *big.Int
}

// NewCommitter takes qOneWay and y generated by the Receiver.
func NewCommitter(qOneWay *RSABased, y *big.Int) (*Committer, error) {
	// y must be from Im(f) where f(x) = x^q mod n, that means gcd(y, n) must be 1:
	// Note that for other q-one-way homomorphisms this validation would be different:
	if !qOneWay.Group.IsElementInGroup(y) {
		return nil, fmt.Errorf("y is not valid")
	}
	return &Committer{
		RSABased: qOneWay,
		Y:        y,
	}, nil
}

func (c *Committer) GetCommitMsg(a *big.Int) (*big.Int, error) {
	if a.Cmp(c.Q) != -1 {
		err := fmt.Errorf("the committed value needs to be < Q")
		return nil, err
	}
	commitment, r := c.computeCommitment(a)
	c.committedValue = a
	c.r = r
	return commitment, nil
}

func (c *Committer) computeCommitment(a *big.Int) (*big.Int, *big.Int) {
	// Y^a * r^Q mod N, where r is random from Z_N*
	r := c.Group.GetRandomElement()
	t1 := c.Group.Exp(c.Y, a)
	t2 := c.Homomorphism(r)
	commitment := c.Group.Mul(t1, t2)
	return commitment, r
}

func (c *Committer) GetDecommitMsg() (*big.Int, *big.Int) {
	return c.committedValue, c.r
}

// GetCommitmentToMultiplication receives a, b, u where u is a random integer used in
// commitment B to b (B = y^b * QOneWayHomomorphism(u)). It returns commitment C to c = a * b mod Q,
// random integer o where C = y^(a*b) * QOneWayHomomorphism(o), and integer t such that
// C = B^a * QOneWayHomomorphism(t).
func (c *Committer) GetCommitmentToMultiplication(a, b, u *big.Int) (*big.Int,
	*big.Int, *big.Int) {
	commitment := new(big.Int).Mul(a, b)
	cMod := new(big.Int).Mod(commitment, c.Q) // c = a * b mod Q
	C, o := c.computeCommitment(cMod)

	j := new(big.Int).Sub(commitment, cMod)
	j.Div(j, c.Q)

	// We want t such that: C = B^a * f(t). We know C = y^(a*b) * f(o) and B^a = (y^a)^b) * f(u)^a.
	// t = o * u^(-a)
	uToa := c.Group.Exp(u, a)
	uToaInv := c.Group.Inv(uToa)
	t := c.Group.Mul(o, uToaInv)

	yToj := c.Group.Exp(c.Y, j)
	yTojInv := c.Group.Inv(yToj)
	t1 := c.HomomorphismInv(yTojInv)
	t = c.Group.Mul(t, t1)
	return C, o, t
}

type Receiver struct {
	*RSABased
	Y          *big.Int
	x          *big.Int
	commitment *big.Int
}

func NewReceiver(nBitLength int) (*Receiver, error) {
	qOneWay, err := NewRSABased(nBitLength)
	if err != nil {
		return nil, err
	}

	// gcd(q, phi(N)) is prime because q is prime and q > N > phi(N).
	// Let's choose some x from Z_n*.
	x := qOneWay.Group.GetRandomElement()
	y := qOneWay.Homomorphism(x)

	return &Receiver{
		RSABased: qOneWay,
		x:        x,
		Y:        y,
	}, nil
}

// When receiver receives a commitment, it stores the value using SetCommitment method.
func (r *Receiver) SetCommitment(c *big.Int) {
	r.commitment = c
}

func (r *Receiver) CheckDecommitment(R, a *big.Int) bool {
	t1 := r.Group.Exp(r.Y, a)
	t2 := r.Group.Exp(R, r.Q)
	c := r.Group.Mul(t1, t2)

	return c.Cmp(r.commitment) == 0
}
