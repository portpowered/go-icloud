package bridgeprover

import (
	"crypto/elliptic"
	"encoding/binary"
	"encoding/hex"
	"math/big"

	model "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgeprover"
)

func decodePoint(value string) (*big.Int, *big.Int, error) {
	encoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, nil, ErrPayload
	}

	curve := elliptic.P256()

	pointX, pointY := elliptic.Unmarshal(curve, encoded)

	if pointX == nil {
		pointX, pointY = elliptic.UnmarshalCompressed(curve, encoded)
	}

	if pointX == nil {
		return nil, nil, ErrPayload
	}

	return pointX, pointY, nil
}

func encodePoint(x, y *big.Int) ([]byte, error) {
	if x == nil || y == nil || !elliptic.P256().IsOnCurve(x, y) {
		return nil, ErrPayload
	}

	return elliptic.Marshal(elliptic.P256(), x, y), nil
}

func scalarBytes(value *big.Int) []byte {
	reduced := new(big.Int).Mod(value, elliptic.P256().Params().N)

	return reduced.Bytes()
}

func (p *Prover) transcript(message string) ([]byte, error) {
	curve := elliptic.P256()

	serverX, serverY, err := decodePoint(message)
	if err != nil {
		return nil, err
	}

	maskNX, maskNY, err := decodePoint(string(model.SPAKEN))
	if err != nil {
		return nil, err
	}

	maskX, maskY := curve.ScalarMult(maskNX, maskNY, scalarBytes(p.w0))
	maskY = new(big.Int).Sub(curve.Params().P, maskY)

	adjustedX, adjustedY := curve.Add(serverX, serverY, maskX, maskY)
	if !curve.IsOnCurve(adjustedX, adjustedY) {
		return nil, ErrPayload
	}

	sharedX, sharedY := curve.ScalarMult(adjustedX, adjustedY, p.x.Bytes())
	verifierX, verifierY := curve.ScalarMult(adjustedX, adjustedY, scalarBytes(p.w1))

	shared, err := encodePoint(sharedX, sharedY)
	if err != nil {
		return nil, err
	}

	verifier, err := encodePoint(verifierX, verifierY)
	if err != nil {
		return nil, err
	}

	mx, my, err := decodePoint(string(model.SPAKEM))
	if err != nil {
		return nil, err
	}

	mPoint, _ := encodePoint(mx, my)
	nPoint, _ := encodePoint(maskNX, maskNY)

	p.shareV, err = hex.DecodeString(message)
	if err != nil {
		return nil, ErrPayload
	}

	server, _ := encodePoint(serverX, serverY)

	return concat(
		[]byte(model.SPAKEContext), []byte(model.ClientIdentity), []byte(model.ServerIdentity),
		mPoint, nPoint, p.shareP, server, shared, verifier, p.w0.Bytes(),
	), nil
}

func concat(parts ...[]byte) []byte {
	output := []byte{}
	for _, part := range parts {
		output = binary.LittleEndian.AppendUint64(output, uint64(len(part)))
		output = append(output, part...)
	}

	return output
}
