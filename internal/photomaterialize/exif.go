package photomaterialize

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"
	"unicode/utf8"
)

const (
	tiffHeaderSize     = 8
	ifdEntrySize       = 12
	ifdSize            = 30
	tagDateTime        = uint16(DateTimeTag)
	tagExifIFD         = uint16(ExifIFDTag)
	tagOriginal        = uint16(OriginalTag)
	tagDigitized       = uint16(DigitizedTag)
	asciiType          = uint16(ASCIIFieldType)
	longType           = uint16(LongFieldType)
	jpegHeaderSize     = 4
	jpegMarkerSize     = 2
	jpegMarkerPrefix   = byte(MarkerPrefix)
	jpegAPP1           = byte(MarkerAPP1)
	dateEntryCount     = 2
	tiffInlineSize     = 4
	exifIdentifierSize = 6
)

// UpdateEXIF prepends Source's timestamp APP1 segment if no EXIF date exists.
// Existing bytes, including other metadata, are preserved. Invalid JPEG input is unchanged.
func UpdateEXIF(data []byte, takenAt time.Time) []byte {
	if !bytes.HasPrefix(data, jpegSOI()) || hasEXIFDate(data) {
		return data
	}

	stamp := append([]byte(takenAt.Format("2006:01:02 15:04:05")), 0)
	payload := append(exifIdentifier(), buildTIFF(stamp)...)

	length := len(payload) + jpegMarkerSize
	if length > math.MaxUint16 {
		return data
	}

	result := make([]byte, 0, len(data)+len(payload)+jpegHeaderSize)
	result = append(result, data[:2]...)
	result = append(result, jpegMarkerPrefix, jpegAPP1)
	result = binary.BigEndian.AppendUint16(result, uint16(length))
	result = append(result, payload...)
	result = append(result, data[2:]...)

	return result
}

func buildTIFF(stamp []byte) []byte {
	dataOffset := tiffHeaderSize + ifdSize
	exifOffset := dataOffset + len(stamp)
	exifData := exifOffset + ifdSize
	result := []byte(LittleEndianSignature)
	result = binary.LittleEndian.AppendUint16(result, uint16(ExifTiffVersion))
	result = binary.LittleEndian.AppendUint32(result, tiffHeaderSize)
	result = binary.LittleEndian.AppendUint16(result, dateEntryCount)
	result = appendEntry(result, tagDateTime, asciiType, tiffOffset(len(stamp)), tiffOffset(dataOffset))
	result = appendEntry(result, tagExifIFD, longType, 1, tiffOffset(exifOffset))
	result = binary.LittleEndian.AppendUint32(result, 0)
	result = append(result, stamp...)
	result = binary.LittleEndian.AppendUint16(result, dateEntryCount)
	result = appendEntry(result, tagOriginal, asciiType, tiffOffset(len(stamp)), tiffOffset(exifData))
	result = appendEntry(result, tagDigitized, asciiType, tiffOffset(len(stamp)), tiffOffset(exifData+len(stamp)))
	result = binary.LittleEndian.AppendUint32(result, 0)
	result = append(result, stamp...)
	result = append(result, stamp...)

	return result
}

func tiffOffset(value int) uint32 {
	if value < 0 || uint64(value) > math.MaxUint32 {
		return 0
	}

	return uint32(value)
}

func appendEntry(data []byte, tag, kind uint16, count, offset uint32) []byte {
	data = binary.LittleEndian.AppendUint16(data, tag)
	data = binary.LittleEndian.AppendUint16(data, kind)
	data = binary.LittleEndian.AppendUint32(data, count)

	return binary.LittleEndian.AppendUint32(data, offset)
}

func exifPayload(data []byte) []byte {
	if len(data) < jpegHeaderSize || !bytes.HasPrefix(data, jpegSOI()) {
		return nil
	}

	for index := 2; index+4 <= len(data); {
		if endJPEGSegment(data[index:]) {
			return nil
		}

		length := int(binary.BigEndian.Uint16(data[index+2:]))
		if length < 2 || length > len(data)-index-2 {
			return nil
		}

		payload := data[index+4 : index+2+length]
		if data[index+1] == jpegAPP1 && bytes.HasPrefix(payload, exifIdentifier()) {
			return payload[exifIdentifierSize:]
		}

		index += jpegMarkerSize + length
	}

	return nil
}

func endJPEGSegment(data []byte) bool {
	return data[0] != jpegMarkerPrefix || data[1] == byte(MarkerEOI) || data[1] == byte(MarkerSOS)
}

func jpegSOI() []byte {
	return []byte{jpegMarkerPrefix, byte(MarkerSOI)}
}

func exifIdentifier() []byte {
	return append([]byte(ExifSignature), 0, 0)
}

func tiffOrder(data []byte) binary.ByteOrder {
	if bytes.HasPrefix(data, []byte(LittleEndianSignature)) {
		return binary.LittleEndian
	}

	if bytes.HasPrefix(data, []byte(BigEndianSignature)) {
		return binary.BigEndian
	}

	return nil
}

func hasEXIFDate(data []byte) bool {
	payload := exifPayload(data)

	order := tiffOrder(payload)

	if len(payload) < tiffHeaderSize || order == nil {
		return false
	}

	entries := tiffEntries(payload, uint64(order.Uint32(payload[4:])), order)
	if asciiPresent(payload, entries[tagDateTime]) {
		return true
	}

	pointer := entries[tagExifIFD]
	if len(pointer) != ifdEntrySize || order.Uint16(pointer[2:]) != longType || order.Uint32(pointer[4:]) != 1 {
		return false
	}

	exif := tiffEntries(payload, uint64(order.Uint32(pointer[8:])), order)

	return asciiPresent(payload, exif[tagOriginal]) || asciiPresent(payload, exif[tagDigitized])
}

func tiffEntries(data []byte, offset uint64, order binary.ByteOrder) map[uint16][]byte {
	if offset+2 > uint64(len(data)) {
		return nil
	}

	count := uint64(order.Uint16(data[offset:]))

	offset += 2

	if count*ifdEntrySize > uint64(len(data))-offset {
		return nil
	}

	entries := make(map[uint16][]byte)

	for range count {
		entry := data[offset : offset+ifdEntrySize]
		entries[order.Uint16(entry)] = entry
		offset += ifdEntrySize
	}

	return entries
}

func asciiPresent(data, entry []byte) bool {
	order := tiffOrder(data)
	if len(entry) != ifdEntrySize || order.Uint16(entry[2:]) != asciiType {
		return false
	}

	count := uint64(order.Uint32(entry[4:]))
	offset := uint64(order.Uint32(entry[8:]))

	if count == 0 {
		return false
	}

	var raw []byte

	if count <= tiffInlineSize {
		// Source packs the inline scalar little endian even in a big endian TIFF.
		inline := binary.LittleEndian.AppendUint32(nil, order.Uint32(entry[8:]))
		raw = inline[:count]
	} else if offset+count <= uint64(len(data)) {
		raw = data[offset : offset+count]
	}

	raw = bytes.TrimRight(raw, "\x00")
	for _, value := range raw {
		if value < utf8.RuneSelf {
			return true
		}
	}

	return false
}
