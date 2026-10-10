package webtransport

import "bytes"

// referenceJSONEscapes preserves JSON values while matching Source's non-HTML string encoding.
// Generated extensible models can escape these characters inside their custom marshalers.
func referenceJSONEscapes(encoded []byte) []byte {
	var output bytes.Buffer

	quoted, escaped := false, false
	for index := 0; index < len(encoded); index++ {
		character := encoded[index]
		if decoded, ok := referenceJSONEscapeCharacter(encoded, index, quoted, escaped); ok {
			output.WriteByte(decoded)
			index += 5

			continue
		}

		output.WriteByte(character)
		if escaped {
			escaped = false

			continue
		}

		switch character {
		case '\\':
			if quoted {
				escaped = true
			}
		case '"':
			quoted = !quoted
		}
	}

	return output.Bytes()
}

func referenceJSONEscapeCharacter(encoded []byte, index int, quoted, escaped bool) (byte, bool) {
	if !quoted || escaped || index+5 >= len(encoded) || encoded[index] != '\\' || encoded[index+1] != 'u' {
		return 0, false
	}

	switch string(encoded[index+2 : index+6]) {
	case "003c", "003C":
		return '<', true
	case "003e", "003E":
		return '>', true
	case "0026":
		return '&', true
	}

	return 0, false
}
