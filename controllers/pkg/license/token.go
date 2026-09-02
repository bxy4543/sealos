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
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	licensev1 "github.com/labring/sealos/controllers/license/api/v1"
	"github.com/labring/sealos/controllers/pkg/crypto"
)

func ParseToken(tokenString string) (*jwt.Token, error) {
	return parseToken(tokenString, GetEncryptionKey())
}

func parseToken(tokenString, encodedKey string) (*jwt.Token, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (any, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				algorithm := "<nil>"
				if token.Method != nil {
					algorithm = token.Method.Alg()
				}
				return nil, fmt.Errorf("unsupported license signing algorithm %q", algorithm)
			}
			decodeKey, err := base64.StdEncoding.DecodeString(encodedKey)
			if err != nil {
				return nil, err
			}
			publicKey, err := crypto.ParseRSAPublicKeyFromAnyPEM(string(decodeKey))
			if err != nil {
				return nil, err
			}
			return publicKey, nil
		})
	if err != nil {
		return nil, err
	}
	return token, nil
}

func GetClaimsFromToken(tokenString string) (*Claims, error) {
	token, err := ParseToken(tokenString)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("license claims type mismatch")
	}
	return claims, nil
}

func GetClaimsFromLicense(license *licensev1.License) (*Claims, error) {
	return GetClaimsFromToken(license.Spec.Token)
}
