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

package df

import (
	"math/big"

	"github.com/emmyzkp/crypto/common"
)

type EqualityProver struct {
	committer1         *Committer
	committer2         *Committer
	challengeSpaceSize int
	r1                 *big.Int
	r21                *big.Int
	r22                *big.Int
}

func NewEqualityProver(committer1, committer2 *Committer,
	challengeSpaceSize int) *EqualityProver {
	return &EqualityProver{
		committer1:         committer1,
		committer2:         committer2,
		challengeSpaceSize: challengeSpaceSize,
	}
}

func (p *EqualityProver) GetProofRandomData() (*big.Int, *big.Int) {
	// r1 from [0, T * 2^(NLength + ChallengeSpaceSize))
	nLen := p.committer1.QRSpecialRSA.N.BitLen()
	exp := big.NewInt(int64(nLen + p.challengeSpaceSize))
	b := new(big.Int).Exp(big.NewInt(2), exp, nil)
	b.Mul(b, p.committer1.T)
	r1 := common.GetRandomInt(b)
	p.r1 = r1

	// r21 from [0, 2^(B + 2*NLength + ChallengeSpaceSize))
	b = new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(
		p.committer1.B+2*nLen+p.challengeSpaceSize)), nil)
	r21 := common.GetRandomInt(b)
	p.r21 = r21

	// r12 from [0, 2^(B + 2*NLength + ChallengeSpaceSize))
	b = new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(
		p.committer1.B+2*nLen+p.challengeSpaceSize)), nil)
	r22 := common.GetRandomInt(b)
	p.r22 = r22
	// G^r1 * H^r12, G^r1 * H^r22
	t1 := p.committer1.ComputeCommit(r1, r21)
	t2 := p.committer2.ComputeCommit(r1, r22)
	return t1, t2
}

func (p *EqualityProver) GetProofData(challenge *big.Int) (*big.Int,
	*big.Int, *big.Int) {
	// s1 = r1 + challenge*a (in Z, not modulo)
	// s21 = r21 + challenge*rr1 (in Z, not modulo)
	// s22 = r21 + challenge*rr2 (in Z, not modulo)
	a, rr1 := p.committer1.GetDecommitMsg()
	_, rr2 := p.committer2.GetDecommitMsg()
	s1 := new(big.Int).Mul(challenge, a)
	s1.Add(s1, p.r1)
	s21 := new(big.Int).Mul(challenge, rr1)
	s21.Add(s21, p.r21)
	s22 := new(big.Int).Mul(challenge, rr2)
	s22.Add(s22, p.r22)
	return s1, s21, s22
}

// EqualityProof presents all three messages in sigma protocol - useful when challenge
// is generated by prover via Fiat-Shamir.
type EqualityProof struct {
	ProofRandomData1 *big.Int
	ProofRandomData2 *big.Int
	Challenge        *big.Int
	ProofData1       *big.Int
	ProofData21      *big.Int
	ProofData22      *big.Int
}

func NewEqualityProof(proofRandomData1, proofRandomData2, challenge, proofData1, proofData21,
	proofData22 *big.Int) *EqualityProof {
	return &EqualityProof{
		ProofRandomData1: proofRandomData1,
		ProofRandomData2: proofRandomData2,
		Challenge:        challenge,
		ProofData1:       proofData1,
		ProofData21:      proofData21,
		ProofData22:      proofData22,
	}
}

type EqualityVerifier struct {
	receiver1          *Receiver
	receiver2          *Receiver
	challengeSpaceSize int
	challenge          *big.Int
	proofRandomData1   *big.Int
	proofRandomData2   *big.Int
}

func NewEqualityVerifier(receiver1, receiver2 *Receiver,
	challengeSpaceSize int) *EqualityVerifier {
	return &EqualityVerifier{
		receiver1:          receiver1,
		receiver2:          receiver2,
		challengeSpaceSize: challengeSpaceSize,
	}
}

func (v *EqualityVerifier) SetProofRandomData(proofRandomData1,
	proofRandomData2 *big.Int) {
	v.proofRandomData1 = proofRandomData1
	v.proofRandomData2 = proofRandomData2
}

func (v *EqualityVerifier) GetChallenge() *big.Int {
	exp := big.NewInt(int64(v.challengeSpaceSize))
	b := new(big.Int).Exp(big.NewInt(2), exp, nil)
	challenge := common.GetRandomInt(b)
	v.challenge = challenge
	return challenge
}

// SetChallenge is used when Fiat-Shamir is used - when challenge is generated using hash by the prover.
func (v *EqualityVerifier) SetChallenge(challenge *big.Int) {
	v.challenge = challenge
}

func (v *EqualityVerifier) Verify(s1, s21, s22 *big.Int) bool {
	// verify proofRandomData1 * v.receiver1.Commitment^challenge = G^s1 * H^s21 mod n1
	// verify proofRandomData2 * v.receiver2.Commitment^challenge = G^s1 * H^s22 mod n2
	left1 := v.receiver1.QRSpecialRSA.Exp(v.receiver1.Commitment, v.challenge)
	left1 = v.receiver1.QRSpecialRSA.Mul(v.proofRandomData1, left1)
	right1 := v.receiver1.ComputeCommit(s1, s21)

	left2 := v.receiver2.QRSpecialRSA.Exp(v.receiver2.Commitment, v.challenge)
	left2 = v.receiver2.QRSpecialRSA.Mul(v.proofRandomData2, left2)
	right2 := v.receiver2.ComputeCommit(s1, s22)
	return left1.Cmp(right1) == 0 && left2.Cmp(right2) == 0
}
