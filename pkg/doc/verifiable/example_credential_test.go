/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifiable_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hyperledger/aries-framework-go/pkg/common/log"
	"github.com/hyperledger/aries-framework-go/pkg/doc/jose"
	"github.com/hyperledger/aries-framework-go/pkg/doc/signature/jsonld"
	"github.com/hyperledger/aries-framework-go/pkg/doc/signature/suite"
	"github.com/hyperledger/aries-framework-go/pkg/doc/signature/suite/bbsblssignature2020"
	"github.com/hyperledger/aries-framework-go/pkg/doc/signature/suite/ed25519signature2018"
	"github.com/hyperledger/aries-framework-go/pkg/doc/signature/suite/jsonwebsignature2020"
	sigverifier "github.com/hyperledger/aries-framework-go/pkg/doc/signature/verifier"
	"github.com/hyperledger/aries-framework-go/pkg/doc/util"
	"github.com/hyperledger/aries-framework-go/pkg/doc/util/signature"
	"github.com/hyperledger/aries-framework-go/pkg/doc/verifiable"
	"github.com/hyperledger/aries-framework-go/pkg/kms"
	spi "github.com/hyperledger/aries-framework-go/spi/log"
)

//nolint:gochecknoglobals
var (
	// Private key generated by ed25519.GenerateKey(rand.Reader).
	issuerPrivKey = ed25519.PrivateKey{72, 67, 163, 188, 235, 199, 239, 146, 129, 52, 228, 34, 44, 106, 23, 144, 189, 57, 115, 171, 4, 217, 54, 121, 41, 155, 251, 83, 1, 240, 238, 65, 234, 100, 192, 93, 251, 181, 198, 73, 122, 220, 27, 48, 93, 73, 166, 33, 152, 140, 168, 36, 9, 205, 59, 161, 137, 7, 164, 9, 176, 252, 1, 171}
	issuerPubKey  = ed25519.PublicKey{234, 100, 192, 93, 251, 181, 198, 73, 122, 220, 27, 48, 93, 73, 166, 33, 152, 140, 168, 36, 9, 205, 59, 161, 137, 7, 164, 9, 176, 252, 1, 171}
	issued        = time.Date(2010, time.January, 1, 19, 23, 24, 0, time.UTC)
	expired       = time.Date(2020, time.January, 1, 19, 23, 24, 0, time.UTC)

	bbsPrivKeyB64 = "PcVroyzTlmnYIIq8In8QOZhpK72AdTjj3EitB9tSNrg"
	bbsPubKeyB64  = "l0Wtf3gy5f140G5vCoCJw2420hwk6Xw65/DX3ycv1W7/eMky8DyExw+o1s2bmq3sEIJatkiN8f5D4k0766x0UvfbupFX+vVkeqnlOvT6o2cag2osQdMFbBQqAybOM4Gm"
)

const vcJSON = `
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://www.w3.org/2018/credentials/examples/v1"
  ],
  "credentialSchema": [],
  "credentialSubject": {
    "degree": {
      "type": "BachelorDegree",
      "university": "MIT"
    },
    "id": "did:example:ebfeb1f712ebc6f1c276e12ec21",
    "name": "Jayden Doe",
    "spouse": "did:example:c276e12ec21ebfeb1f712ebc6f1"
  },
  "expirationDate": "2020-01-01T19:23:24Z",
  "id": "http://example.edu/credentials/1872",
  "issuanceDate": "2009-01-01T19:23:24Z",
  "issuer": {
    "id": "did:example:76e12ec712ebc6f1c221ebfeb1f",
    "name": "Example University"
  },
  "referenceNumber": 83294849,
  "type": [
    "VerifiableCredential",
    "UniversityDegreeCredential"
  ]
}
`

func ExampleCredential_embedding() {
	vc := &UniversityDegreeCredential{
		Credential: &verifiable.Credential{
			Context: []string{
				"https://www.w3.org/2018/credentials/v1",
				"https://www.w3.org/2018/credentials/examples/v1",
			},
			ID: "http://example.edu/credentials/1872",
			Types: []string{
				"VerifiableCredential",
				"UniversityDegreeCredential",
			},
			Subject: UniversityDegreeSubject{
				ID:     "did:example:ebfeb1f712ebc6f1c276e12ec21",
				Name:   "Jayden Doe",
				Spouse: "did:example:c276e12ec21ebfeb1f712ebc6f1",
				Degree: UniversityDegree{
					Type:       "BachelorDegree",
					University: "MIT",
				},
			},
			Issuer: verifiable.Issuer{
				ID:           "did:example:76e12ec712ebc6f1c221ebfeb1f",
				CustomFields: verifiable.CustomFields{"name": "Example University"},
			},
			Issued:  util.NewTime(issued),
			Expired: util.NewTime(expired),
			Schemas: []verifiable.TypedID{},
		},
		ReferenceNumber: 83294847,
	}

	// Marshal to JSON to verify the result of decoding.
	vcBytes, err := json.Marshal(vc)
	if err != nil {
		panic("failed to marshal VC to JSON")
	}

	fmt.Println(string(vcBytes))

	// Marshal to JWS.
	jwtClaims, err := vc.JWTClaims(true)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JWT claims of VC: %w", err))
	}

	signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	jws, err := jwtClaims.MarshalJWS(verifiable.EdDSA, signer, "")
	if err != nil {
		panic(fmt.Errorf("failed to sign VC inside JWT: %w", err))
	}

	fmt.Println(jws)

	// Parse JWS and make sure it's coincide with JSON.
	vcParsed, err := verifiable.ParseCredential(
		[]byte(jws),
		verifiable.WithPublicKeyFetcher(verifiable.SingleKey(issuerPubKey, kms.ED25519)),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to encode VC from JWS: %w", err))
	}

	vcBytesFromJWS, err := vcParsed.MarshalJSON()
	if err != nil {
		panic(fmt.Errorf("failed to marshal VC: %w", err))
	}

	// todo missing referenceNumber here (https://github.com/hyperledger/aries-framework-go/issues/847)
	fmt.Println(string(vcBytesFromJWS))

	// Output:
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
	// eyJhbGciOiJFZERTQSIsImtpZCI6IiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1Nzc5MDY2MDQsImlhdCI6MTI2MjM3MzgwNCwiaXNzIjoiZGlkOmV4YW1wbGU6NzZlMTJlYzcxMmViYzZmMWMyMjFlYmZlYjFmIiwianRpIjoiaHR0cDovL2V4YW1wbGUuZWR1L2NyZWRlbnRpYWxzLzE4NzIiLCJuYmYiOjEyNjIzNzM4MDQsInN1YiI6ImRpZDpleGFtcGxlOmViZmViMWY3MTJlYmM2ZjFjMjc2ZTEyZWMyMSIsInZjIjp7IkBjb250ZXh0IjpbImh0dHBzOi8vd3d3LnczLm9yZy8yMDE4L2NyZWRlbnRpYWxzL3YxIiwiaHR0cHM6Ly93d3cudzMub3JnLzIwMTgvY3JlZGVudGlhbHMvZXhhbXBsZXMvdjEiXSwiY3JlZGVudGlhbFN1YmplY3QiOnsiZGVncmVlIjp7InR5cGUiOiJCYWNoZWxvckRlZ3JlZSIsInVuaXZlcnNpdHkiOiJNSVQifSwiaWQiOiJkaWQ6ZXhhbXBsZTplYmZlYjFmNzEyZWJjNmYxYzI3NmUxMmVjMjEiLCJuYW1lIjoiSmF5ZGVuIERvZSIsInNwb3VzZSI6ImRpZDpleGFtcGxlOmMyNzZlMTJlYzIxZWJmZWIxZjcxMmViYzZmMSJ9LCJpc3N1ZXIiOnsibmFtZSI6IkV4YW1wbGUgVW5pdmVyc2l0eSJ9LCJ0eXBlIjpbIlZlcmlmaWFibGVDcmVkZW50aWFsIiwiVW5pdmVyc2l0eURlZ3JlZUNyZWRlbnRpYWwiXX19.7He-0-kAUCgjgMUSI-BmH-9MjI-ixuMV6NUnJCtfLpoOJIkdK0Tf1iU6SWGSURpv67Mi91H-pzQCmW6jzEUABQ
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"type":["VerifiableCredential","UniversityDegreeCredential"]}
}

func ExampleCredential_extraFields() {
	vc := &verifiable.Credential{
		Context: []string{
			"https://www.w3.org/2018/credentials/v1",
			"https://www.w3.org/2018/credentials/examples/v1",
		},
		ID: "http://example.edu/credentials/1872",
		Types: []string{
			"VerifiableCredential",
			"UniversityDegreeCredential",
		},
		Subject: UniversityDegreeSubject{
			ID:     "did:example:ebfeb1f712ebc6f1c276e12ec21",
			Name:   "Jayden Doe",
			Spouse: "did:example:c276e12ec21ebfeb1f712ebc6f1",
			Degree: UniversityDegree{
				Type:       "BachelorDegree",
				University: "MIT",
			},
		},
		Issuer: verifiable.Issuer{
			ID:           "did:example:76e12ec712ebc6f1c221ebfeb1f",
			CustomFields: verifiable.CustomFields{"name": "Example University"},
		},
		Issued:  util.NewTime(issued),
		Expired: util.NewTime(expired),
		Schemas: []verifiable.TypedID{},
		CustomFields: map[string]interface{}{
			"referenceNumber": 83294847,
		},
	}

	// Marshal to JSON.
	vcBytes, err := json.Marshal(vc)
	if err != nil {
		panic("failed to marshal VC to JSON")
	}

	fmt.Println(string(vcBytes))

	// Marshal to JWS.
	jwtClaims, err := vc.JWTClaims(true)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JWT claims of VC: %w", err))
	}

	signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	jws, err := jwtClaims.MarshalJWS(verifiable.EdDSA, signer, "")
	if err != nil {
		panic(fmt.Errorf("failed to sign VC inside JWT: %w", err))
	}

	fmt.Println(jws)

	// Parse JWS and make sure it's coincide with JSON.
	vcParsed, err := verifiable.ParseCredential(
		[]byte(jws),
		verifiable.WithPublicKeyFetcher(verifiable.SingleKey(issuerPubKey, kms.ED25519)),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to encode VC from JWS: %w", err))
	}

	vcBytesFromJWS, err := vcParsed.MarshalJSON()
	if err != nil {
		panic(fmt.Errorf("failed to marshal VC: %w", err))
	}

	fmt.Println(string(vcBytesFromJWS))

	// Output:
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
	// eyJhbGciOiJFZERTQSIsImtpZCI6IiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1Nzc5MDY2MDQsImlhdCI6MTI2MjM3MzgwNCwiaXNzIjoiZGlkOmV4YW1wbGU6NzZlMTJlYzcxMmViYzZmMWMyMjFlYmZlYjFmIiwianRpIjoiaHR0cDovL2V4YW1wbGUuZWR1L2NyZWRlbnRpYWxzLzE4NzIiLCJuYmYiOjEyNjIzNzM4MDQsInN1YiI6ImRpZDpleGFtcGxlOmViZmViMWY3MTJlYmM2ZjFjMjc2ZTEyZWMyMSIsInZjIjp7IkBjb250ZXh0IjpbImh0dHBzOi8vd3d3LnczLm9yZy8yMDE4L2NyZWRlbnRpYWxzL3YxIiwiaHR0cHM6Ly93d3cudzMub3JnLzIwMTgvY3JlZGVudGlhbHMvZXhhbXBsZXMvdjEiXSwiY3JlZGVudGlhbFN1YmplY3QiOnsiZGVncmVlIjp7InR5cGUiOiJCYWNoZWxvckRlZ3JlZSIsInVuaXZlcnNpdHkiOiJNSVQifSwiaWQiOiJkaWQ6ZXhhbXBsZTplYmZlYjFmNzEyZWJjNmYxYzI3NmUxMmVjMjEiLCJuYW1lIjoiSmF5ZGVuIERvZSIsInNwb3VzZSI6ImRpZDpleGFtcGxlOmMyNzZlMTJlYzIxZWJmZWIxZjcxMmViYzZmMSJ9LCJpc3N1ZXIiOnsibmFtZSI6IkV4YW1wbGUgVW5pdmVyc2l0eSJ9LCJyZWZlcmVuY2VOdW1iZXIiOjguMzI5NDg0N2UrMDcsInR5cGUiOlsiVmVyaWZpYWJsZUNyZWRlbnRpYWwiLCJVbml2ZXJzaXR5RGVncmVlQ3JlZGVudGlhbCJdfX0.a5yKMPmDnEXvM-fG3BaOqfdkqdvU4s2rzeZuOzLmkTH1y9sJT-mgTe7map5E9x7abrNVpyYbaH7JaAb9Yhr1DQ
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
}

func ExampleParseCredential() {
	// Issuer is about to issue the university degree credential for the Holder
	vcEncoded := &verifiable.Credential{
		Context: []string{
			"https://www.w3.org/2018/credentials/v1",
			"https://www.w3.org/2018/credentials/examples/v1",
		},
		ID: "http://example.edu/credentials/1872",
		Types: []string{
			"VerifiableCredential",
			"UniversityDegreeCredential",
		},
		Subject: UniversityDegreeSubject{
			ID:     "did:example:ebfeb1f712ebc6f1c276e12ec21",
			Name:   "Jayden Doe",
			Spouse: "did:example:c276e12ec21ebfeb1f712ebc6f1",
			Degree: UniversityDegree{
				Type:       "BachelorDegree",
				University: "MIT",
			},
		},
		Issuer: verifiable.Issuer{
			ID:           "did:example:76e12ec712ebc6f1c221ebfeb1f",
			CustomFields: verifiable.CustomFields{"name": "Example University"},
		},
		Issued:  util.NewTime(issued),
		Expired: util.NewTime(expired),
		Schemas: []verifiable.TypedID{},
		CustomFields: map[string]interface{}{
			"referenceNumber": 83294847,
		},
	}

	// ... in JWS form.
	jwtClaims, err := vcEncoded.JWTClaims(true)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JWT claims of VC: %w", err))
	}

	signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	jws, err := jwtClaims.MarshalJWS(verifiable.EdDSA, signer, "")
	if err != nil {
		panic(fmt.Errorf("failed to sign VC inside JWT: %w", err))
	}

	// The Holder receives JWS and decodes it.
	vcParsed, err := verifiable.ParseCredential(
		[]byte(jws),
		verifiable.WithPublicKeyFetcher(verifiable.SingleKey(issuerPubKey, kms.ED25519)),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to decode VC JWS: %w", err))
	}

	vcDecodedBytes, err := vcParsed.MarshalJSON()
	if err != nil {
		panic(fmt.Errorf("failed to marshal VC: %w", err))
	}

	// The Holder then e.g. can save the credential to her personal verifiable credential wallet.
	fmt.Println(string(vcDecodedBytes))

	// Output: {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
}

func ExampleCredential_JWTClaims() {
	// The Holder wants to send the credential to the Verifier in JWS.
	vc, err := verifiable.ParseCredential([]byte(vcJSON),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to decode VC JSON: %w", err))
	}

	jwtClaims, err := vc.JWTClaims(true)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JWT claims of VC: %w", err))
	}

	signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	jws, err := jwtClaims.MarshalJWS(verifiable.EdDSA, signer, "")
	if err != nil {
		panic(fmt.Errorf("failed to sign VC inside JWT: %w", err))
	}

	// The Holder passes JWS to Verifier
	fmt.Println(jws)

	// Output: eyJhbGciOiJFZERTQSIsImtpZCI6IiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1Nzc5MDY2MDQsImlhdCI6MTIzMDgzNzgwNCwiaXNzIjoiZGlkOmV4YW1wbGU6NzZlMTJlYzcxMmViYzZmMWMyMjFlYmZlYjFmIiwianRpIjoiaHR0cDovL2V4YW1wbGUuZWR1L2NyZWRlbnRpYWxzLzE4NzIiLCJuYmYiOjEyMzA4Mzc4MDQsInN1YiI6ImRpZDpleGFtcGxlOmViZmViMWY3MTJlYmM2ZjFjMjc2ZTEyZWMyMSIsInZjIjp7IkBjb250ZXh0IjpbImh0dHBzOi8vd3d3LnczLm9yZy8yMDE4L2NyZWRlbnRpYWxzL3YxIiwiaHR0cHM6Ly93d3cudzMub3JnLzIwMTgvY3JlZGVudGlhbHMvZXhhbXBsZXMvdjEiXSwiY3JlZGVudGlhbFN1YmplY3QiOnsiZGVncmVlIjp7InR5cGUiOiJCYWNoZWxvckRlZ3JlZSIsInVuaXZlcnNpdHkiOiJNSVQifSwiaWQiOiJkaWQ6ZXhhbXBsZTplYmZlYjFmNzEyZWJjNmYxYzI3NmUxMmVjMjEiLCJuYW1lIjoiSmF5ZGVuIERvZSIsInNwb3VzZSI6ImRpZDpleGFtcGxlOmMyNzZlMTJlYzIxZWJmZWIxZjcxMmViYzZmMSJ9LCJpc3N1ZXIiOnsibmFtZSI6IkV4YW1wbGUgVW5pdmVyc2l0eSJ9LCJyZWZlcmVuY2VOdW1iZXIiOjguMzI5NDg0OWUrMDcsInR5cGUiOlsiVmVyaWZpYWJsZUNyZWRlbnRpYWwiLCJVbml2ZXJzaXR5RGVncmVlQ3JlZGVudGlhbCJdfX0.9-hiifM2cvfAcK6Olk5JSEnhlcRAAe0LYlpZW4nHat_3jVP4rjvKhP6bLNfTEkJ0271-NZZRd0YsI9Dg_-uKAg
}

func ExampleCredential_AddLinkedDataProof() {
	vc, err := verifiable.ParseCredential([]byte(vcJSON),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to decode VC JSON: %w", err))
	}

	signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	err = vc.AddLinkedDataProof(&verifiable.LinkedDataProofContext{
		Created:                 &issued,
		SignatureType:           "Ed25519Signature2018",
		Suite:                   ed25519signature2018.New(suite.WithSigner(signer)),
		SignatureRepresentation: verifiable.SignatureJWS,
		VerificationMethod:      "did:example:123456#key1",
	}, jsonld.WithDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to add linked data proof: %w", err))
	}

	vcJSONWithProof, err := json.MarshalIndent(vc, "", "\t")
	if err != nil {
		panic(fmt.Errorf("failed to marshal VC to JSON: %w", err))
	}

	fmt.Println(string(vcJSONWithProof))

	// Output: {
	//	"@context": [
	//		"https://www.w3.org/2018/credentials/v1",
	//		"https://www.w3.org/2018/credentials/examples/v1"
	//	],
	//	"credentialSubject": {
	//		"degree": {
	//			"type": "BachelorDegree",
	//			"university": "MIT"
	//		},
	//		"id": "did:example:ebfeb1f712ebc6f1c276e12ec21",
	//		"name": "Jayden Doe",
	//		"spouse": "did:example:c276e12ec21ebfeb1f712ebc6f1"
	//	},
	//	"expirationDate": "2020-01-01T19:23:24Z",
	//	"id": "http://example.edu/credentials/1872",
	//	"issuanceDate": "2009-01-01T19:23:24Z",
	//	"issuer": {
	//		"id": "did:example:76e12ec712ebc6f1c221ebfeb1f",
	//		"name": "Example University"
	//	},
	//	"proof": {
	//		"created": "2010-01-01T19:23:24Z",
	//		"jws": "eyJhbGciOiJFZERTQSIsImI2NCI6ZmFsc2UsImNyaXQiOlsiYjY0Il19..lrkhpRH4tWl6KzQKHlcyAwSm8qUTXIMSKmD3QASF_uI5QW8NWLxLebXmnQpIM8H7umhLA6dINSYVowcaPdpwBw",
	//		"proofPurpose": "assertionMethod",
	//		"type": "Ed25519Signature2018",
	//		"verificationMethod": "did:example:123456#key1"
	//	},
	//	"referenceNumber": 83294849,
	//	"type": [
	//		"VerifiableCredential",
	//		"UniversityDegreeCredential"
	//	]
	//}
}

//nolint:govet
func ExampleCredential_AddLinkedDataProofMultiProofs() {
	log.SetLevel("aries-framework/json-ld-processor", spi.ERROR)

	vc, err := verifiable.ParseCredential([]byte(vcJSON),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(fmt.Errorf("failed to decode VC JSON: %w", err))
	}

	ed25519Signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	err = vc.AddLinkedDataProof(&verifiable.LinkedDataProofContext{
		Created:                 &issued,
		SignatureType:           "Ed25519Signature2018",
		Suite:                   ed25519signature2018.New(suite.WithSigner(ed25519Signer)),
		SignatureRepresentation: verifiable.SignatureJWS,
		VerificationMethod:      "did:example:123456#key1",
	}, jsonld.WithDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(err)
	}

	ecdsaSigner, err := signature.NewSigner(kms.ECDSASecp256k1TypeIEEEP1363)
	if err != nil {
		panic(err)
	}

	err = vc.AddLinkedDataProof(&verifiable.LinkedDataProofContext{
		Created:                 &issued,
		SignatureType:           "JsonWebSignature2020",
		Suite:                   jsonwebsignature2020.New(suite.WithSigner(ecdsaSigner)),
		SignatureRepresentation: verifiable.SignatureJWS,
		VerificationMethod:      "did:example:123456#key2",
	}, jsonld.WithDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(err)
	}

	vcBytes, err := json.Marshal(vc)
	if err != nil {
		panic(err)
	}

	// Verify the VC with two embedded proofs.
	ed25519Suite := ed25519signature2018.New(suite.WithVerifier(ed25519signature2018.NewPublicKeyVerifier()))
	jsonWebSignatureSuite := jsonwebsignature2020.New(suite.WithVerifier(jsonwebsignature2020.NewPublicKeyVerifier()))

	jwk, err := jose.JWKFromKey(ecdsaSigner.PublicKey())
	if err != nil {
		panic(err)
	}

	_, err = verifiable.ParseCredential(vcBytes,
		verifiable.WithEmbeddedSignatureSuites(ed25519Suite, jsonWebSignatureSuite),
		verifiable.WithPublicKeyFetcher(func(issuerID, keyID string) (*sigverifier.PublicKey, error) {
			switch keyID {
			case "#key1":
				return &sigverifier.PublicKey{
					Type:  "Ed25519Signature2018",
					Value: issuerPubKey,
				}, nil

			case "#key2":
				return &sigverifier.PublicKey{
					Type:  "JwsVerificationKey2020",
					Value: ecdsaSigner.PublicKeyBytes(),
					JWK:   jwk,
				}, nil
			}

			return nil, errors.New("unsupported keyID")
		}),
		verifiable.WithJSONLDOnlyValidRDF(),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(err)
	}
	// Output:
}

//nolint:gocyclo
func ExampleCredential_GenerateBBSSelectiveDisclosure() {
	log.SetLevel("aries-framework/json-ld-processor", spi.ERROR)

	vcStr := `
	{
	 "@context": [
	   "https://www.w3.org/2018/credentials/v1",
	   "https://w3id.org/citizenship/v1",
	   "https://w3id.org/security/bbs/v1"
	 ],
	 "id": "https://issuer.oidp.uscis.gov/credentials/83627465",
	 "type": [
	   "VerifiableCredential",
	   "PermanentResidentCard"
	 ],
	 "issuer": "did:example:b34ca6cd37bbf23",
	 "identifier": "83627465",
	 "name": "Permanent Resident Card",
	 "description": "Government of Example Permanent Resident Card.",
	 "issuanceDate": "2019-12-03T12:19:52Z",
	 "expirationDate": "2029-12-03T12:19:52Z",
	 "credentialSubject": {
	   "id": "did:example:b34ca6cd37bbf23",
	   "type": [
	     "PermanentResident",
	     "Person"
	   ],
	   "givenName": "JOHN",
	   "familyName": "SMITH",
	   "gender": "Male",
	   "image": "data:image/png;base64,iVBORw0KGgokJggg==",
	   "residentSince": "2015-01-01",
	   "lprCategory": "C09",
	   "lprNumber": "999-999-999",
	   "commuterClassification": "C1",
	   "birthCountry": "Bahamas",
	   "birthDate": "1958-07-17"
	 }
	}
`

	vc, err := verifiable.ParseCredential([]byte(vcStr),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()),
		verifiable.WithDisabledProofCheck())
	if err != nil {
		panic(fmt.Errorf("failed to decode VC JSON: %w", err))
	}

	ed25519Signer := signature.GetEd25519Signer(issuerPrivKey, issuerPubKey)

	err = vc.AddLinkedDataProof(&verifiable.LinkedDataProofContext{
		Created:                 &issued,
		SignatureType:           "Ed25519Signature2018",
		Suite:                   ed25519signature2018.New(suite.WithSigner(ed25519Signer)),
		SignatureRepresentation: verifiable.SignatureJWS,
		VerificationMethod:      "did:example:123456#key1",
	}, jsonld.WithDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(err)
	}

	pubKey, privKey, err := loadBBSKeyPair(bbsPubKeyB64, bbsPrivKeyB64)
	if err != nil {
		panic(err)
	}

	bbsSigner, err := newBBSSigner(privKey)
	if err != nil {
		panic(err)
	}

	err = vc.AddLinkedDataProof(&verifiable.LinkedDataProofContext{
		Created:                 &issued,
		SignatureType:           "BbsBlsSignature2020",
		Suite:                   bbsblssignature2020.New(suite.WithSigner(bbsSigner)),
		SignatureRepresentation: verifiable.SignatureProofValue,
		VerificationMethod:      "did:example:123456#key1",
	}, jsonld.WithDocumentLoader(getJSONLDDocumentLoader()))
	if err != nil {
		panic(err)
	}

	// BBS+ signature is generated each time unique, that's why we substitute it with some constant value
	// for a reason of keeping constant test output.
	originalProofValue := hideProofValue(vc.Proofs[1], "dummy signature value")

	vcJSONWithProof, err := json.MarshalIndent(vc, "", "\t")
	if err != nil {
		panic(fmt.Errorf("failed to marshal VC to JSON: %w", err))
	}

	fmt.Println(string(vcJSONWithProof))

	restoreProofValue(vc.Proofs[1], originalProofValue)

	// Create BBS+ selective disclosure. We explicitly state the fields we want to reveal in the output document.
	// For example, "credentialSubject.birthDate" is not mentioned and thus will be hidden.
	// To hide top-level VC fields, "@explicit": true is used on top level of reveal doc.
	// For example, we can reveal "identifier" top-level VC field only. "issuer" and "issuanceDate" are mandatory
	// and thus must be defined in reveal doc in case of hiding top-level VC fields.
	revealDoc := `
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://w3id.org/citizenship/v1",
    "https://w3id.org/security/bbs/v1"
  ],
  "type": ["VerifiableCredential", "PermanentResidentCard"],
  "@explicit": true,
  "identifier": {},
  "issuer": {},
  "issuanceDate": {},
  "credentialSubject": {
    "@explicit": true,
    "type": ["PermanentResident", "Person"],
    "givenName": {},
    "familyName": {},
    "gender": {}
  }
}
`

	var revealDocMap map[string]interface{}

	err = json.Unmarshal([]byte(revealDoc), &revealDocMap)
	if err != nil {
		panic(err)
	}

	pubKeyBytes, err := pubKey.Marshal()
	if err != nil {
		panic(err)
	}

	vcWithSelectiveDisclosure, err := vc.GenerateBBSSelectiveDisclosure(revealDocMap, []byte("some nonce"),
		verifiable.WithJSONLDDocumentLoader(getJSONLDDocumentLoader()),
		verifiable.WithPublicKeyFetcher(verifiable.SingleKey(pubKeyBytes, "Bls12381G2Key2020")))
	if err != nil {
		panic(err)
	}

	// Only BBS+ related proof left.
	hideProofValue(vcWithSelectiveDisclosure.Proofs[0], "dummy signature proof value")

	vcJSONWithProof, err = json.MarshalIndent(vcWithSelectiveDisclosure, "", "\t")
	if err != nil {
		panic(fmt.Errorf("failed to marshal VC to JSON: %w", err))
	}

	fmt.Println()
	fmt.Println(string(vcJSONWithProof))
	// Output:{
	//	"@context": [
	//		"https://www.w3.org/2018/credentials/v1",
	//		"https://w3id.org/citizenship/v1",
	//		"https://w3id.org/security/bbs/v1"
	//	],
	//	"credentialSubject": {
	//		"birthCountry": "Bahamas",
	//		"birthDate": "1958-07-17",
	//		"commuterClassification": "C1",
	//		"familyName": "SMITH",
	//		"gender": "Male",
	//		"givenName": "JOHN",
	//		"id": "did:example:b34ca6cd37bbf23",
	//		"image": "data:image/png;base64,iVBORw0KGgokJggg==",
	//		"lprCategory": "C09",
	//		"lprNumber": "999-999-999",
	//		"residentSince": "2015-01-01",
	//		"type": [
	//			"PermanentResident",
	//			"Person"
	//		]
	//	},
	//	"description": "Government of Example Permanent Resident Card.",
	//	"expirationDate": "2029-12-03T12:19:52Z",
	//	"id": "https://issuer.oidp.uscis.gov/credentials/83627465",
	//	"identifier": "83627465",
	//	"issuanceDate": "2019-12-03T12:19:52Z",
	//	"issuer": "did:example:b34ca6cd37bbf23",
	//	"name": "Permanent Resident Card",
	//	"proof": [
	//		{
	//			"created": "2010-01-01T19:23:24Z",
	//			"jws": "eyJhbGciOiJFZERTQSIsImI2NCI6ZmFsc2UsImNyaXQiOlsiYjY0Il19..HsBapUAZDdaZZy6hrn951768kJaRmNAwTWvVnTDM-Bp5k08eEnnxrii5n47AeWVLDJJo7P0dEPafyC_gMjFPAA",
	//			"proofPurpose": "assertionMethod",
	//			"type": "Ed25519Signature2018",
	//			"verificationMethod": "did:example:123456#key1"
	//		},
	//		{
	//			"created": "2010-01-01T19:23:24Z",
	//			"proofPurpose": "assertionMethod",
	//			"proofValue": "ZHVtbXkgc2lnbmF0dXJlIHZhbHVl",
	//			"type": "BbsBlsSignature2020",
	//			"verificationMethod": "did:example:123456#key1"
	//		}
	//	],
	//	"type": [
	//		"VerifiableCredential",
	//		"PermanentResidentCard"
	//	]
	//}
	//
	//{
	//	"@context": [
	//		"https://www.w3.org/2018/credentials/v1",
	//		"https://w3id.org/citizenship/v1",
	//		"https://w3id.org/security/bbs/v1"
	//	],
	//	"credentialSubject": {
	//		"familyName": "SMITH",
	//		"gender": "Male",
	//		"givenName": "JOHN",
	//		"id": "did:example:b34ca6cd37bbf23",
	//		"type": [
	//			"Person",
	//			"PermanentResident"
	//		]
	//	},
	//	"id": "https://issuer.oidp.uscis.gov/credentials/83627465",
	//	"identifier": "83627465",
	//	"issuanceDate": "2019-12-03T12:19:52Z",
	//	"issuer": "did:example:b34ca6cd37bbf23",
	//	"proof": {
	//		"created": "2010-01-01T19:23:24Z",
	//		"nonce": "c29tZSBub25jZQ==",
	//		"proofPurpose": "assertionMethod",
	//		"proofValue": "ZHVtbXkgc2lnbmF0dXJlIHByb29mIHZhbHVl",
	//		"type": "BbsBlsSignatureProof2020",
	//		"verificationMethod": "did:example:123456#key1"
	//	},
	//	"type": [
	//		"PermanentResidentCard",
	//		"VerifiableCredential"
	//	]
	//}
}

func hideProofValue(proof verifiable.Proof, dummyValue string) interface{} {
	oldProofValue := proof["proofValue"]
	proof["proofValue"] = base64.StdEncoding.EncodeToString([]byte(dummyValue))

	return oldProofValue
}

func restoreProofValue(proof verifiable.Proof, proofValue interface{}) {
	proof["proofValue"] = proofValue
}
