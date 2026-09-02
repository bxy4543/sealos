// Copyright © 2026 sealos.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package license

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	licensev1 "github.com/labring/sealos/controllers/license/api/v1"
)

func testEncodedPublicKey(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	publicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKey,
	}))
}

func testSignedToken(t *testing.T, method jwt.SigningMethod, privateKey *rsa.PrivateKey, expiresAt time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(method, &Claims{
		Type: licensev1.ClusterLicenseType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestParseTokenAcceptsRS256(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	encodedKey := testEncodedPublicKey(t, privateKey)
	signed := testSignedToken(t, jwt.SigningMethodRS256, privateKey, time.Now().Add(time.Hour))

	token, err := parseToken(signed, encodedKey)
	if err != nil {
		t.Fatalf("parse RS256 token: %v", err)
	}
	if !token.Valid {
		t.Fatal("RS256 token is not valid")
	}
}

func TestParseTokenRejectsUnsupportedAlgorithm(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		Type: licensev1.ClusterLicenseType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign HS256 token: %v", err)
	}

	if _, err := parseToken(signed, testEncodedPublicKey(t, privateKey)); err == nil {
		t.Fatal("unsupported HS256 token was accepted")
	}
}

func TestParseTokenRejectsExpiredToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	signed := testSignedToken(t, jwt.SigningMethodRS256, privateKey, time.Now().Add(-time.Hour))

	if _, err := parseToken(signed, testEncodedPublicKey(t, privateKey)); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestParseTokenRejectsInvalidSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate expected RSA key: %v", err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing RSA key: %v", err)
	}
	signed := testSignedToken(t, jwt.SigningMethodRS256, otherKey, time.Now().Add(time.Hour))

	if _, err := parseToken(signed, testEncodedPublicKey(t, privateKey)); err == nil {
		t.Fatal("token with invalid signature was accepted")
	}
}
