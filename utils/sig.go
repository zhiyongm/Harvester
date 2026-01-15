package utils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"math/big"
	"time"

	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	mr "math/rand"
)

func generateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	return privKey, &privKey.PublicKey, nil
}

func signData(privKey *rsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.New()
	_, err := hash.Write(data)
	if err != nil {
		return nil, err
	}

	hashed := hash.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hashed)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

func verifySignature(pubKey *rsa.PublicKey, data, signature []byte) error {
	hash := sha256.New()
	_, err := hash.Write(data)
	if err != nil {
		return err
	}

	hashed := hash.Sum(nil)

	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed, signature)
	if err != nil {
		return fmt.Errorf("签名验证失败: %v", err)
	}

	return nil
}

type SignSimulator struct {
	privKey    *rsa.PrivateKey
	pubKey     *rsa.PublicKey
	signeddata []byte
}

func (RSASimulator *SignSimulator) SimulateVerify() {
	signeddata_ := RSASimulator.signeddata
	pk := RSASimulator.pubKey
	verifySignature(pk, []byte("Hello, World!"), signeddata_)

}

func NewRSASimulator() *SignSimulator {
	privKey, pubKey, err := generateKeyPair(2048)
	if err != nil {
		return nil
	}
	signeddata, err := signData(privKey, []byte("Hello, World!"))
	return &SignSimulator{
		privKey:    privKey,
		pubKey:     pubKey,
		signeddata: signeddata,
	}

}

type ECDSASimulator struct {
	privKey    *ecdsa.PrivateKey
	pubKey     *ecdsa.PublicKey
	signeddata []byte
	r          *big.Int
	s          *big.Int

	pri_eddsa_key       ed25519.PrivateKey
	pub_eddsa_key       ed25519.PublicKey
	ed25519_signed_data []byte
	mrr                 *mr.Rand
}

func (ECDSASimulator *ECDSASimulator) SimulateECDSAVerify() {
	msg := sha256.Sum256([]byte("1"))
	pubKey := ECDSASimulator.pubKey

	if ecdsa.Verify(pubKey, msg[:], ECDSASimulator.r, ECDSASimulator.s) {
	} else {
		fmt.Println("signature failed")
	}
}

func (ECDSASimulator *ECDSASimulator) SimulateED25519Verify() {

	msg := sha256.Sum256([]byte("1"))
	pubKey := ECDSASimulator.pub_eddsa_key
	ed25519.Verify(pubKey, msg[:], ECDSASimulator.ed25519_signed_data)
}
func NewECDSASimulator() *ECDSASimulator {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	pubkley, prikey, _ := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		panic(err)
		return nil
	}

	msg := sha256.Sum256([]byte("1"))
	r, s, _ := ecdsa.Sign(rand.Reader, privKey, msg[:])

	ed25519_signed_data := ed25519.Sign(prikey, msg[:])

	return &ECDSASimulator{
		privKey:             privKey,
		pubKey:              &privKey.PublicKey,
		r:                   r,
		s:                   s,
		ed25519_signed_data: ed25519_signed_data,
		pub_eddsa_key:       pubkley,
		mrr:                 mr.New(mr.NewSource(time.Now().UnixNano())),
	}

}

func GenerateRandomSig(rand2 *mr.Rand) []byte {
	randomBytes := make([]byte, 65)

	for i := 0; i < 65; i++ {
		randomBytes[i] = byte(rand2.Intn(256))
	}

	return randomBytes
}
