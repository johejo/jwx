package jws_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha512"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"

	"github.com/lestrrat-go/jwx/internal/json"
	"github.com/lestrrat-go/jwx/internal/jwxtest"

	"github.com/lestrrat-go/jwx/buffer"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jws/sign"
	"github.com/lestrrat-go/jwx/jws/verify"
	"github.com/stretchr/testify/assert"
)

const examplePayload = `{"iss":"joe",` + "\r\n" + ` "exp":1300819380,` + "\r\n" + ` "http://example.com/is_root":true}`
const exampleCompactSerialization = `eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk`
const badValue = "%badvalue%"

func TestParse(t *testing.T) {
	t.Parallel()
	t.Run("Empty bytes.Buffer", func(t *testing.T) {
		t.Parallel()
		_, err := jws.Parse(&bytes.Buffer{})
		if !assert.Error(t, err, "Parsing an empty buffer should result in an error") {
			return
		}
	})
	t.Run("Compact missing parts", func(t *testing.T) {
		t.Parallel()
		incoming := strings.Join(
			(strings.Split(
				exampleCompactSerialization,
				".",
			))[:2],
			".",
		)
		_, err := jws.ParseString(incoming)
		if !assert.Error(t, err, "Parsing compact serialization with less than 3 parts should be an error") {
			return
		}
	})
	t.Run("Compact bad header", func(t *testing.T) {
		t.Parallel()
		parts := strings.Split(exampleCompactSerialization, ".")
		parts[0] = badValue
		incoming := strings.Join(parts, ".")

		_, err := jws.ParseString(incoming)
		if !assert.Error(t, err, "Parsing compact serialization with bad header should be an error") {
			return
		}
	})
	t.Run("Compact bad payload", func(t *testing.T) {
		t.Parallel()
		parts := strings.Split(exampleCompactSerialization, ".")
		parts[1] = badValue
		incoming := strings.Join(parts, ".")

		_, err := jws.ParseString(incoming)
		if !assert.Error(t, err, "Parsing compact serialization with bad payload should be an error") {
			return
		}
	})
	t.Run("Compact bad signature", func(t *testing.T) {
		t.Parallel()
		parts := strings.Split(exampleCompactSerialization, ".")
		parts[2] = badValue
		incoming := strings.Join(parts, ".")

		t.Logf("incoming = '%s'", incoming)
		_, err := jws.ParseString(incoming)
		if !assert.Error(t, err, "Parsing compact serialization with bad signature should be an error") {
			return
		}
	})
}

func TestRoundtrip(t *testing.T) {
	t.Parallel()
	payload := []byte("Lorem ipsum")
	sharedkey := []byte("Avracadabra")

	hmacAlgorithms := []jwa.SignatureAlgorithm{jwa.HS256, jwa.HS384, jwa.HS512}
	for _, alg := range hmacAlgorithms {
		alg := alg
		t.Run("HMAC "+alg.String(), func(t *testing.T) {
			t.Parallel()
			signed, err := jws.Sign(payload, alg, sharedkey)
			if !assert.NoError(t, err, "Sign succeeds") {
				return
			}

			verified, err := jws.Verify(signed, alg, sharedkey)
			if !assert.NoError(t, err, "Verify succeeded") {
				return
			}

			if !assert.Equal(t, payload, verified, "verified payload matches") {
				return
			}
		})
	}
	t.Run("HMAC SignMulti", func(t *testing.T) {
		var signed []byte
		t.Run("Sign", func(t *testing.T) {
			var options []jws.Option
			for _, alg := range hmacAlgorithms {
				signer, err := sign.New(alg)
				if !assert.NoError(t, err, `sign.New should succeed`) {
					return
				}
				options = append(options, jws.WithSigner(signer, sharedkey, nil, nil))
			}
			var err error
			signed, err = jws.SignMulti(payload, options...)
			if !assert.NoError(t, err, `jws.SignMulti should succeed`) {
				return
			}
		})
		for _, alg := range hmacAlgorithms {
			alg := alg
			t.Run("Verify "+alg.String(), func(t *testing.T) {
				verified, err := jws.Verify(signed, alg, sharedkey)
				if !assert.NoError(t, err, "Verify succeeded") {
					return
				}

				if !assert.Equal(t, payload, verified, "verified payload matches") {
					return
				}
			})
		}
	})
}

func TestVerifyWithJWKSet(t *testing.T) {
	payload := []byte("Hello, World!")
	key, err := jwxtest.GenerateRsaKey()
	if !assert.NoError(t, err, "RSA key generated") {
		return
	}

	jwkKey, err := jwk.New(&key.PublicKey)
	if !assert.NoError(t, err, "JWK public key generated") {
		return
	}
	err = jwkKey.Set(jwk.AlgorithmKey, jwa.RS256)
	if !assert.NoError(t, err, "Algorithm set successfully") {
		return
	}

	buf, err := jws.Sign(payload, jwa.RS256, key)
	if !assert.NoError(t, err, "Signature generated successfully") {
		return
	}

	verified, err := jws.VerifyWithJWKSet(buf, &jwk.Set{Keys: []jwk.Key{jwkKey}}, nil)
	if !assert.NoError(t, err, "Verify is successful") {
		return
	}
	if !assert.Equal(t, payload, verified, "Verified payload is the same") {
		return
	}

	verified, err = jws.VerifyWithJWK(buf, jwkKey)
	if !assert.NoError(t, err, "Verify is successful") {
		return
	}

	if !assert.Equal(t, payload, verified, "Verified payload is the same") {
		return
	}

	key2, err := jwxtest.GenerateRsaKey()
	if !assert.NoError(t, err, "RSA key generated") {
		return
	}
	jwkKey2, err := jwk.New(&key2.PublicKey)
	if !assert.NoError(t, err, "JWK Public key generated") {
		return
	}
	_, err = jws.VerifyWithJWKSet(buf, &jwk.Set{Keys: []jwk.Key{jwkKey2}}, nil)
	if !assert.Error(t, err, "Verify with wrong key should fail") {
		return
	}
}

func TestRoundtrip_RSACompact(t *testing.T) {
	payload := []byte("Hello, World!")
	for _, alg := range []jwa.SignatureAlgorithm{jwa.RS256, jwa.RS384, jwa.RS512, jwa.PS256, jwa.PS384, jwa.PS512} {
		key, err := jwxtest.GenerateRsaKey()
		if !assert.NoError(t, err, "RSA key generated") {
			return
		}

		buf, err := jws.Sign(payload, alg, key)
		if !assert.NoError(t, err, "(%s) Signature generated successfully", alg) {
			return
		}

		parsers := map[string]func([]byte) (*jws.Message, error){
			"Parse(io.Reader)": func(b []byte) (*jws.Message, error) { return jws.Parse(bytes.NewReader(b)) },
			"Parse(string)":    func(b []byte) (*jws.Message, error) { return jws.ParseString(string(b)) },
		}
		for name, f := range parsers {
			m, err := f(buf)
			if !assert.NoError(t, err, "(%s) %s is successful", alg, name) {
				return
			}

			if !assert.Equal(t, payload, m.Payload(), "(%s) %s: Payload is decoded", alg, name) {
				return
			}
		}

		verified, err := jws.Verify(buf, alg, &key.PublicKey)
		if !assert.NoError(t, err, "(%s) Verify is successful", alg) {
			return
		}

		if !assert.Equal(t, payload, verified, "(%s) Verified payload is the same", alg) {
			return
		}
	}
}

func TestEncode(t *testing.T) {
	t.Parallel()

	// HS256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.1 works
	t.Run("HS256Compact", func(t *testing.T) {
		t.Parallel()
		const hdr = `{"typ":"JWT",` + "\r\n" + ` "alg":"HS256"}`
		const hmacKey = `AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow`
		const expected = `eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk`

		hmacKeyDecoded := buffer.Buffer{}
		err := hmacKeyDecoded.Base64Decode([]byte(hmacKey))
		if !assert.NoError(t, err, "HMAC base64 decoded successful") {
			return
		}

		hdrbuf, err := buffer.Buffer(hdr).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}
		payload, err := buffer.Buffer(examplePayload).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		signingInput := bytes.Join(
			[][]byte{
				hdrbuf,
				payload,
			},
			[]byte{'.'},
		)

		sign, err := sign.New(jwa.HS256)
		if !assert.NoError(t, err, "HMAC signer created successfully") {
			return
		}

		signature, err := sign.Sign(signingInput, hmacKeyDecoded.Bytes())
		if !assert.NoError(t, err, "PayloadSign is successful") {
			return
		}
		sigbuf, err := buffer.Buffer(signature).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		encoded := bytes.Join(
			[][]byte{
				signingInput,
				sigbuf,
			},
			[]byte{'.'},
		)
		if !assert.Equal(t, expected, string(encoded), "generated compact serialization should match") {
			return
		}

		msg, err := jws.Parse(bytes.NewReader(encoded))
		if !assert.NoError(t, err, "Parsing compact encoded serialization succeeds") {
			return
		}

		signatures := msg.Signatures()
		if !assert.Len(t, signatures, 1, `there should be exactly one signature`) {
			return
		}

		algorithm := signatures[0].ProtectedHeaders().Algorithm()
		if algorithm != jwa.HS256 {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.HS256)
		if !assert.NoError(t, err, "HmacVerify created") {
			return
		}

		if !assert.NoError(t, v.Verify(signingInput, signature, hmacKeyDecoded.Bytes()), "Verify succeeds") {
			return
		}
	})
	t.Run("HS256CompactLiteral", func(t *testing.T) {
		t.Parallel()
		const hdr = `{"typ":"JWT",` + "\r\n" + ` "alg":"HS256"}`
		const jwksrc = `{
"kty":"oct",
"k":"AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
}`

		hdrBytes := []byte(hdr)

		hdrBuf, err := buffer.Buffer(hdr).Base64Encode()
		if err != nil {
			t.Fatal("Failed to base64 encode protected header")
		}
		standardHeaders := jws.NewHeaders()
		err = json.Unmarshal(hdrBytes, standardHeaders)
		if err != nil {
			t.Fatal("Failed to parse protected header")
		}
		alg := standardHeaders.Algorithm()

		payload, err := buffer.Buffer(examplePayload).Base64Encode()
		if err != nil {
			t.Fatal("Failed to base64 encode payload")
		}

		keys, _ := jwk.ParseString(jwksrc)
		var key interface{}
		if !assert.NoError(t, keys.Keys[0].Raw(&key), `jwk.Raw should succeed`) {
			return
		}
		var jwsCompact []byte
		jwsCompact, err = jws.SignLiteral([]byte(examplePayload), alg, key, hdrBytes)
		if err != nil {
			t.Fatal("Failed to sign message")
		}

		msg, err := jws.Parse(bytes.NewReader(jwsCompact))
		if !assert.NoError(t, err, "Parsing compact encoded serialization succeeds") {
			return
		}

		signatures := msg.Signatures()
		if !assert.Len(t, signatures, 1, `there should be exactly one signature`) {
			return
		}

		algorithm := signatures[0].ProtectedHeaders().Algorithm()
		if algorithm != alg {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(alg)
		if !assert.NoError(t, err, "HmacVerify created") {
			return
		}

		signingInput := bytes.Join(
			[][]byte{
				hdrBuf,
				payload,
			},
			[]byte{'.'},
		)

		if !assert.NoError(t, v.Verify(signingInput, signatures[0].Signature(), key), "Verify succeeds") {
			return
		}
	})
	t.Run("ES512Compact", func(t *testing.T) {
		t.Parallel()
		// ES256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.3 works
		hdr := []byte{123, 34, 97, 108, 103, 34, 58, 34, 69, 83, 53, 49, 50, 34, 125}
		const jwksrc = `{
"kty":"EC",
"crv":"P-521",
"x":"AekpBQ8ST8a8VcfVOTNl353vSrDCLLJXmPk06wTjxrrjcBpXp5EOnYG_NjFZ6OvLFV1jSfS9tsz4qUxcWceqwQGk",
"y":"ADSmRA43Z1DSNx_RvcLI87cdL07l6jQyyBXMoxVg_l2Th-x3S1WDhjDly79ajL4Kkd0AZMaZmh9ubmf63e3kyMj2",
"d":"AY5pb7A0UFiB3RELSD64fTLOSV_jazdF7fLYyuTw8lOfRhWg6Y6rUrPAxerEzgdRhajnu0ferB0d53vM9mE15j2C"
}`

		// "Payload"
		jwsPayload := []byte{80, 97, 121, 108, 111, 97, 100}

		standardHeaders := jws.NewHeaders()
		if !assert.NoError(t, json.Unmarshal(hdr, standardHeaders), `parsing headers should succeed`) {
			return
		}

		alg := standardHeaders.Algorithm()

		keys, err := jwk.ParseString(jwksrc)
		if err != nil {
			t.Fatal("Failed to parse JWK")
		}
		var key interface{}
		if !assert.NoError(t, keys.Keys[0].Raw(&key), `jwk.Raw should succeed`) {
			return
		}
		var jwsCompact []byte
		jwsCompact, err = jws.Sign(jwsPayload, alg, key)
		if err != nil {
			t.Fatal("Failed to sign message")
		}

		// Verify with standard ecdsa library
		_, _, jwsSignature, err := jws.SplitCompact(bytes.NewReader(jwsCompact))
		if err != nil {
			t.Fatal("Failed to split compact JWT")
		}
		decodedJwsSignature := make([]byte, base64.RawURLEncoding.DecodedLen(len(jwsSignature)))
		decodedLen, err := base64.RawURLEncoding.Decode(decodedJwsSignature, jwsSignature)
		if err != nil {
			t.Fatal("Failed to sign message")
		}
		r, s := &big.Int{}, &big.Int{}
		n := decodedLen / 2
		r.SetBytes(decodedJwsSignature[:n])
		s.SetBytes(decodedJwsSignature[n:])
		signingHdr, err := buffer.Buffer(hdr).Base64Encode()
		if err != nil {
			t.Fatal("Failed to base64 encode headers")
		}
		signingPayload, err := buffer.Buffer(jwsPayload).Base64Encode()
		if err != nil {
			t.Fatal("Failed to base64 encode payload")
		}
		jwsSigningInput := bytes.Join(
			[][]byte{
				signingHdr,
				signingPayload,
			},
			[]byte{'.'},
		)
		hashed512 := sha512.Sum512(jwsSigningInput)
		ecdsaPrivateKey := key.(*ecdsa.PrivateKey)
		if !assert.True(t, ecdsa.Verify(&ecdsaPrivateKey.PublicKey, hashed512[:], r, s), "ecdsa.Verify should succeed") {
			return
		}

		// Verify with API library
		publicKey, err := jwk.PublicKeyOf(key)
		if !assert.NoError(t, err, `jwk.PublicKeyOf should succeed`) {
			return
		}
		verifiedPayload, err := jws.Verify(jwsCompact, alg, publicKey)
		if err != nil || string(verifiedPayload) != string(jwsPayload) {
			t.Fatal("Failed to verify message")
		}
	})
	t.Run("RS256Compact", func(t *testing.T) {
		t.Parallel()
		// RS256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.2 works
		const hdr = `{"alg":"RS256"}`
		const expected = `eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.cC4hiUPoj9Eetdgtv3hF80EGrhuB__dzERat0XF9g2VtQgr9PJbu3XOiZj5RZmh7AAuHIm4Bh-0Qc_lF5YKt_O8W2Fp5jujGbds9uJdbF9CUAr7t1dnZcAcQjbKBYNX4BAynRFdiuB--f_nZLgrnbyTyWzO75vRK5h6xBArLIARNPvkSjtQBMHlb1L07Qe7K0GarZRmB_eSN9383LcOLn6_dO--xi12jzDwusC-eOkHWEsqtFZESc6BfI7noOPqvhJ1phCnvWh6IeYI2w9QOYEUipUTI8np6LbgGY9Fs98rqVt5AXLIhWkWywlVmtVrBp0igcN_IoypGlUPQGe77Rw`
		const jwksrc = `{
    "kty":"RSA",
    "n":"ofgWCuLjybRlzo0tZWJjNiuSfb4p4fAkd_wWJcyQoTbji9k0l8W26mPddxHmfHQp-Vaw-4qPCJrcS2mJPMEzP1Pt0Bm4d4QlL-yRT-SFd2lZS-pCgNMsD1W_YpRPEwOWvG6b32690r2jZ47soMZo9wGzjb_7OMg0LOL-bSf63kpaSHSXndS5z5rexMdbBYUsLA9e-KXBdQOS-UTo7WTBEMa2R2CapHg665xsmtdVMTBQY4uDZlxvb3qCo5ZwKh9kG4LT6_I5IhlJH7aGhyxXFvUK-DWNmoudF8NAco9_h9iaGNj8q2ethFkMLs91kzk2PAcDTW9gb54h4FRWyuXpoQ",
    "e":"AQAB",
    "d":"Eq5xpGnNCivDflJsRQBXHx1hdR1k6Ulwe2JZD50LpXyWPEAeP88vLNO97IjlA7_GQ5sLKMgvfTeXZx9SE-7YwVol2NXOoAJe46sui395IW_GO-pWJ1O0BkTGoVEn2bKVRUCgu-GjBVaYLU6f3l9kJfFNS3E0QbVdxzubSu3Mkqzjkn439X0M_V51gfpRLI9JYanrC4D4qAdGcopV_0ZHHzQlBjudU2QvXt4ehNYTCBr6XCLQUShb1juUO1ZdiYoFaFQT5Tw8bGUl_x_jTj3ccPDVZFD9pIuhLhBOneufuBiB4cS98l2SR_RQyGWSeWjnczT0QU91p1DhOVRuOopznQ",
    "p":"4BzEEOtIpmVdVEZNCqS7baC4crd0pqnRH_5IB3jw3bcxGn6QLvnEtfdUdiYrqBdss1l58BQ3KhooKeQTa9AB0Hw_Py5PJdTJNPY8cQn7ouZ2KKDcmnPGBY5t7yLc1QlQ5xHdwW1VhvKn-nXqhJTBgIPgtldC-KDV5z-y2XDwGUc",
    "q":"uQPEfgmVtjL0Uyyx88GZFF1fOunH3-7cepKmtH4pxhtCoHqpWmT8YAmZxaewHgHAjLYsp1ZSe7zFYHj7C6ul7TjeLQeZD_YwD66t62wDmpe_HlB-TnBA-njbglfIsRLtXlnDzQkv5dTltRJ11BKBBypeeF6689rjcJIDEz9RWdc",
    "dp":"BwKfV3Akq5_MFZDFZCnW-wzl-CCo83WoZvnLQwCTeDv8uzluRSnm71I3QCLdhrqE2e9YkxvuxdBfpT_PI7Yz-FOKnu1R6HsJeDCjn12Sk3vmAktV2zb34MCdy7cpdTh_YVr7tss2u6vneTwrA86rZtu5Mbr1C1XsmvkxHQAdYo0",
    "dq":"h_96-mK1R_7glhsum81dZxjTnYynPbZpHziZjeeHcXYsXaaMwkOlODsWa7I9xXDoRwbKgB719rrmI2oKr6N3Do9U0ajaHF-NKJnwgjMd2w9cjz3_-kyNlxAr2v4IKhGNpmM5iIgOS1VZnOZ68m6_pbLBSp3nssTdlqvd0tIiTHU",
    "qi":"IYd7DHOhrWvxkwPQsRM2tOgrjbcrfvtQJipd-DlcxyVuuM9sQLdgjVk2oy26F0EmpScGLq2MowX7fhd_QJQ3ydy5cY7YIBi87w93IKLEdfnbJtoOPLUW0ITrJReOgo1cq9SbsxYawBgfp_gh6A5603k2-ZQwVK0JKSHuLFkuQ3U"
  }`

		privkey := jwk.NewRSAPrivateKey()
		if !assert.NoError(t, json.Unmarshal([]byte(jwksrc), privkey), `parsing jwk should be successful`) {
			return
		}

		var rawkey rsa.PrivateKey
		if !assert.NoError(t, privkey.Raw(&rawkey), `obtaining raw key should succeed`) {
			return
		}

		sign, err := sign.New(jwa.RS256)
		if !assert.NoError(t, err, "RsaSign created successfully") {
			return
		}

		hdrbuf, err := buffer.Buffer(hdr).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}
		payload, err := buffer.Buffer(examplePayload).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		signingInput := bytes.Join(
			[][]byte{
				hdrbuf,
				payload,
			},
			[]byte{'.'},
		)
		signature, err := sign.Sign(signingInput, rawkey)
		if !assert.NoError(t, err, "PayloadSign is successful") {
			return
		}
		sigbuf, err := buffer.Buffer(signature).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		encoded := bytes.Join(
			[][]byte{
				signingInput,
				sigbuf,
			},
			[]byte{'.'},
		)

		if !assert.Equal(t, expected, string(encoded), "generated compact serialization should match") {
			return
		}

		msg, err := jws.Parse(bytes.NewReader(encoded))
		if !assert.NoError(t, err, "Parsing compact encoded serialization succeeds") {
			return
		}

		signatures := msg.Signatures()
		if !assert.Len(t, signatures, 1, `there should be exactly one signature`) {
			return
		}

		algorithm := signatures[0].ProtectedHeaders().Algorithm()
		if algorithm != jwa.RS256 {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.RS256)
		if !assert.NoError(t, err, "Verify created") {
			return
		}

		if !assert.NoError(t, v.Verify(signingInput, signature, rawkey.PublicKey), "Verify succeeds") {
			return
		}
	})
	t.Run("ES256Compact", func(t *testing.T) {
		t.Parallel()
		// ES256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.3 works
		const hdr = `{"alg":"ES256"}`
		const jwksrc = `{
    "kty":"EC",
    "crv":"P-256",
    "x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",
    "y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0",
    "d":"jpsQnnGQmL-YBIffH1136cspYG6-0iY7X1fCE9-E9LI"
  }`
		privkey := jwk.NewECDSAPrivateKey()
		if !assert.NoError(t, json.Unmarshal([]byte(jwksrc), privkey), `parsing jwk should succeed`) {
			return
		}

		var rawkey ecdsa.PrivateKey
		if !assert.NoError(t, privkey.Raw(&rawkey), `obtaining raw key should succeed`) {
			return
		}

		signer, err := sign.New(jwa.ES256)
		if !assert.NoError(t, err, "RsaSign created successfully") {
			return
		}

		hdrbuf, err := buffer.Buffer(hdr).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}
		payload, err := buffer.Buffer(examplePayload).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		signingInput := bytes.Join(
			[][]byte{
				hdrbuf,
				payload,
			},
			[]byte{'.'},
		)
		signature, err := signer.Sign(signingInput, &rawkey)
		if !assert.NoError(t, err, "PayloadSign is successful") {
			return
		}
		sigbuf, err := buffer.Buffer(signature).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		encoded := bytes.Join(
			[][]byte{
				signingInput,
				sigbuf,
			},
			[]byte{'.'},
		)

		// The signature contains random factor, so unfortunately we can't match
		// the output against a fixed expected outcome. We'll wave doing an
		// exact match, and just try to verify using the signature

		msg, err := jws.Parse(bytes.NewReader(encoded))
		if !assert.NoError(t, err, "Parsing compact encoded serialization succeeds") {
			return
		}

		signatures := msg.Signatures()
		if !assert.Len(t, signatures, 1, `there should be exactly one signature`) {
			return
		}

		algorithm := signatures[0].ProtectedHeaders().Algorithm()
		if algorithm != jwa.ES256 {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.ES256)
		if !assert.NoError(t, err, "EcdsaVerify created") {
			return
		}
		if !assert.NoError(t, v.Verify(signingInput, signature, rawkey.PublicKey), "Verify succeeds") {
			return
		}
	})
	t.Run("EdDSACompact", func(t *testing.T) {
		t.Parallel()
		// EdDSACompact tests that https://tools.ietf.org/html/rfc8037#appendix-A.1-5 works
		const hdr = `{"alg":"EdDSA"}`
		const jwksrc = `{
    "kty":"OKP",
    "crv":"Ed25519",
    "d":"nWGxne_9WmC6hEr0kuwsxERJxWl7MmkZcDusAxyuf2A",
    "x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"
  }`
		const examplePayload = `Example of Ed25519 signing`
		const expected = `hgyY0il_MGCjP0JzlnLWG1PPOt7-09PGcvMg3AIbQR6dWbhijcNR4ki4iylGjg5BhVsPt9g7sVvpAr_MuM0KAg`
		expectedDecoded := buffer.Buffer{}
		err := expectedDecoded.Base64Decode([]byte(expected))
		if !assert.NoError(t, err, "Expected Signature decode successful") {
			return
		}

		privkey := jwk.NewOKPPrivateKey()
		if !assert.NoError(t, json.Unmarshal([]byte(jwksrc), privkey), `parsing jwk should succeed`) {
			return
		}

		var rawkey ed25519.PrivateKey
		if !assert.NoError(t, privkey.Raw(&rawkey), `obtaining raw key should succeed`) {
			return
		}

		signer, err := sign.New(jwa.EdDSA)
		if !assert.NoError(t, err, "EdDSASign created successfully") {
			return
		}

		hdrbuf, err := buffer.Buffer(hdr).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}
		payload, err := buffer.Buffer(examplePayload).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		signingInput := bytes.Join(
			[][]byte{
				hdrbuf,
				payload,
			},
			[]byte{'.'},
		)
		signature, err := signer.Sign(signingInput, rawkey)
		if !assert.NoError(t, err, "PayloadSign is successful") {
			return
		}
		sigbuf, err := buffer.Buffer(signature).Base64Encode()
		if !assert.NoError(t, err, "base64 encode successful") {
			return
		}

		encoded := bytes.Join(
			[][]byte{
				signingInput,
				sigbuf,
			},
			[]byte{'.'},
		)

		// The signature contains random factor, so unfortunately we can't match
		// the output against a fixed expected outcome. We'll wave doing an
		// exact match, and just try to verify using the signature

		msg, err := jws.Parse(bytes.NewReader(encoded))
		if !assert.NoError(t, err, "Parsing compact encoded serialization succeeds") {
			return
		}

		signatures := msg.Signatures()
		if !assert.Len(t, signatures, 1, `there should be exactly one signature`) {
			return
		}

		algorithm := signatures[0].ProtectedHeaders().Algorithm()
		if algorithm != jwa.EdDSA {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.EdDSA)
		if !assert.NoError(t, err, "EcdsaVerify created") {
			return
		}
		if !assert.NoError(t, v.Verify(signingInput, signature, rawkey.Public()), "Verify succeeds") {
			return
		}
		if !assert.Equal(t, signature, expectedDecoded.Bytes(), "signatures match") {
			return
		}
	})
	t.Run("UnsecuredCompact", func(t *testing.T) {
		t.Parallel()
		s := `eyJhbGciOiJub25lIn0.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.`

		m, err := jws.Parse(strings.NewReader(s))
		if !assert.NoError(t, err, "Parsing compact serialization") {
			return
		}

		{
			v := map[string]interface{}{}
			if !assert.NoError(t, json.Unmarshal(m.Payload(), &v), "Unmarshal payload") {
				return
			}
			if !assert.Equal(t, v["iss"], "joe", "iss matches") {
				return
			}
			if !assert.Equal(t, int(v["exp"].(float64)), 1300819380, "exp matches") {
				return
			}
			if !assert.Equal(t, v["http://example.com/is_root"], true, "'http://example.com/is_root' matches") {
				return
			}
		}

		if !assert.Len(t, m.Signatures(), 1, "There should be 1 signature") {
			return
		}

		signatures := m.Signatures()
		algorithm := signatures[0].ProtectedHeaders().Algorithm()
		if algorithm != jwa.NoSignature {
			t.Fatal("Algorithm in header does not match")
		}

		if !assert.Empty(t, signatures[0].Signature(), "Signature should be empty") {
			return
		}
	})
	t.Run("CompleteJSON", func(t *testing.T) {
		t.Parallel()
		s := `{
    "payload": "eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ",
    "signatures":[
      {
        "header": {"kid":"2010-12-29"},
        "protected":"eyJhbGciOiJSUzI1NiJ9",
        "signature": "cC4hiUPoj9Eetdgtv3hF80EGrhuB__dzERat0XF9g2VtQgr9PJbu3XOiZj5RZmh7AAuHIm4Bh-0Qc_lF5YKt_O8W2Fp5jujGbds9uJdbF9CUAr7t1dnZcAcQjbKBYNX4BAynRFdiuB--f_nZLgrnbyTyWzO75vRK5h6xBArLIARNPvkSjtQBMHlb1L07Qe7K0GarZRmB_eSN9383LcOLn6_dO--xi12jzDwusC-eOkHWEsqtFZESc6BfI7noOPqvhJ1phCnvWh6IeYI2w9QOYEUipUTI8np6LbgGY9Fs98rqVt5AXLIhWkWywlVmtVrBp0igcN_IoypGlUPQGe77Rw"
      },
      {
        "header": {"kid":"e9bc097a-ce51-4036-9562-d2ade882db0d"},
        "protected":"eyJhbGciOiJFUzI1NiJ9",
        "signature": "DtEhU3ljbEg8L38VWAfUAqOyKAM6-Xx-F4GawxaepmXFCgfTjDxw5djxLa8ISlSApmWQxfKTUJqPP3-Kg6NU1Q"
      }
    ]
  }`

		m, err := jws.Parse(strings.NewReader(s))
		if !assert.NoError(t, err, "Unmarshal complete json serialization") {
			return
		}

		t.Logf("%#v", m)
		if !assert.Len(t, m.Signatures(), 2, "There should be 2 signatures") {
			return
		}

		sigs := m.LookupSignature("2010-12-29")
		if !assert.Len(t, sigs, 1, "There should be 1 signature with kid = '2010-12-29'") {
			return
		}
	})
	t.Run("Protected Header lookup", func(t *testing.T) {
		t.Parallel()
		s := `{
    "payload": "eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ",
    "signatures":[
      {
        "header": {"cty":"example"},
        "protected":"eyJhbGciOiJFUzI1NiIsImtpZCI6ImU5YmMwOTdhLWNlNTEtNDAzNi05NTYyLWQyYWRlODgyZGIwZCJ9",
        "signature": "JcLb1udPAV72TayGv6eawZKlIQQ3K1NzB0fU7wwYoFypGxEczdCQU-V9jp4WwY2ueJKYeE4fF6jigB0PdSKR0Q"
      }
    ]
  }`

		// Protected Header is {"alg":"ES256","kid":"e9bc097a-ce51-4036-9562-d2ade882db0d"}
		// This protected header combination forces the parser/unmarshal to go trough the code path to populate and look for protected header fields.
		// The signature is valid.

		m, err := jws.Parse(strings.NewReader(s))
		if !assert.NoError(t, err, "Unmarshal complete json serialization") {
			return
		}
		if len(m.Signatures()) != 1 {
			t.Fatal("There should be 1 signature")
		}

		sigs := m.LookupSignature("e9bc097a-ce51-4036-9562-d2ade882db0d")
		if !assert.Len(t, sigs, 1, "There should be 1 signature with kid = '2010-12-29'") {
			return
		}
	})
	t.Run("FlattenedJSON", func(t *testing.T) {
		t.Parallel()
		s := `{
    "payload": "eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ",
    "protected":"eyJhbGciOiJFUzI1NiJ9",
    "header": {
      "kid":"e9bc097a-ce51-4036-9562-d2ade882db0d"
    },
    "signature": "DtEhU3ljbEg8L38VWAfUAqOyKAM6-Xx-F4GawxaepmXFCgfTjDxw5djxLa8ISlSApmWQxfKTUJqPP3-Kg6NU1Q"
  }`

		m, err := jws.Parse(strings.NewReader(s))
		if !assert.NoError(t, err, "Parsing flattened json serialization") {
			return
		}

		if !assert.Len(t, m.Signatures(), 1, "There should be 1 signature") {
			return
		}

		jsonbuf, _ := json.MarshalIndent(m, "", "  ")
		t.Logf("%s", jsonbuf)
	})
	t.Run("SplitCompact short", func(t *testing.T) {
		t.Parallel()
		// Create string with X.Y.Z
		numX := 100
		numY := 100
		numZ := 100
		var largeString = ""
		for i := 0; i < numX; i++ {
			largeString += "X"
		}
		largeString += "."
		for i := 0; i < numY; i++ {
			largeString += "Y"
		}
		largeString += "."
		for i := 0; i < numZ; i++ {
			largeString += "Z"
		}
		x, y, z, err := jws.SplitCompact(strings.NewReader(largeString))
		if !assert.NoError(t, err, "SplitCompactShort string split") {
			return
		}
		if !assert.Len(t, x, numX, "Length of header") {
			return
		}
		if !assert.Len(t, y, numY, "Length of payload") {
			return
		}
		if !assert.Len(t, z, numZ, "Length of signature") {
			return
		}
	})
	t.Run("SplitCompact long", func(t *testing.T) {
		t.Parallel()
		// Create string with X.Y.Z
		numX := 8000
		numY := 8000
		numZ := 8000
		var largeString = ""
		for i := 0; i < numX; i++ {
			largeString += "X"
		}
		largeString += "."
		for i := 0; i < numY; i++ {
			largeString += "Y"
		}
		largeString += "."
		for i := 0; i < numZ; i++ {
			largeString += "Z"
		}
		x, y, z, err := jws.SplitCompact(strings.NewReader(largeString))
		if !assert.NoError(t, err, "SplitCompactShort string split") {
			return
		}
		if !assert.Len(t, x, numX, "Length of header") {
			return
		}
		if !assert.Len(t, y, numY, "Length of payload") {
			return
		}
		if !assert.Len(t, z, numZ, "Length of signature") {
			return
		}
	})
}

func TestPublicHeaders(t *testing.T) {
	key, err := jwxtest.GenerateRsaKey()
	if !assert.NoError(t, err, "GenerateKey should succeed") {
		return
	}

	signer, err := sign.New(jwa.RS256)
	if !assert.NoError(t, err, "rsasign.NewSigner should succeed") {
		return
	}
	_ = signer // TODO

	pubkey := key.PublicKey
	pubjwk, err := jwk.New(&pubkey)
	if !assert.NoError(t, err, "NewRsaPublicKey should succeed") {
		return
	}
	_ = pubjwk // TODO
}

func TestDecode_ES384Compact_NoSigTrim(t *testing.T) {
	incoming := "eyJhbGciOiJFUzM4NCIsInR5cCI6IkpXVCIsImtpZCI6IjE5MzFmZTQ0YmFhMWNhZTkyZWUzNzYzOTQ0MDU1OGMwODdlMTRlNjk5ZWU5NjVhM2Q1OGU1MmU2NGY4MDE0NWIifQ.eyJpc3MiOiJicmt0LWNsaS0xLjAuN3ByZTEiLCJpYXQiOjE0ODQ2OTU1MjAsImp0aSI6IjgxYjczY2Y3In0.DdFi0KmPHSv4PfIMGcWGMSRLmZsfRPQ3muLFW6Ly2HpiLFFQWZ0VEanyrFV263wjlp3udfedgw_vrBLz3XC8CkbvCo_xeHMzaTr_yfhjoheSj8gWRLwB-22rOnUX_M0A"
	t.Logf("incoming = '%s'", incoming)
	const jwksrc = `{
    "kty":"EC",
    "crv":"P-384",
    "x":"YHVZ4gc1RDoqxKm4NzaN_Y1r7R7h3RM3JMteC478apSKUiLVb4UNytqWaLoE6ygH",
    "y":"CRKSqP-aYTIsqJfg_wZEEYUayUR5JhZaS2m4NLk2t1DfXZgfApAJ2lBO0vWKnUMp"
  }`

	pubkey := jwk.NewECDSAPublicKey()
	if !assert.NoError(t, json.Unmarshal([]byte(jwksrc), pubkey), `parsing jwk should be successful`) {
		return
	}

	var rawkey ecdsa.PublicKey
	if !assert.NoError(t, pubkey.Raw(&rawkey), `obtaining raw key should succeed`) {
		return
	}

	v, err := verify.New(jwa.ES384)
	if !assert.NoError(t, err, "EcdsaVerify created") {
		return
	}

	protected, payload, signature, err := jws.SplitCompact(strings.NewReader(incoming))
	if !assert.NoError(t, err, `jws.SplitCompact should succeed`) {
		return
	}

	var buf bytes.Buffer
	buf.Write(protected)
	buf.WriteByte('.')
	buf.Write(payload)

	decodedSignature := make([]byte, base64.RawURLEncoding.DecodedLen(len(signature)))
	if _, err := base64.RawURLEncoding.Decode(decodedSignature, signature); !assert.NoError(t, err, `decoding signature should succeed`) {
		return
	}

	if !assert.NoError(t, v.Verify(buf.Bytes(), decodedSignature, rawkey), "Verify succeeds") {
		return
	}
}

func TestGHIssue126(t *testing.T) {
	_, err := jws.Verify([]byte("{}"), jwa.ES384, nil)
	if !assert.Error(t, err, "Verify should fail") {
		return
	}

	if !assert.Equal(t, err.Error(), `failed to unmarshal JSON message: "payload" must be non-empty`) {
		return
	}
}
