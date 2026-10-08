package protocol

// HTTP compression framing values follow RFC 1950 and RFC 1952.
const (
	GzipHeaderLength     = 10
	GzipMagicFirst       = 0x1f
	GzipMagicSecond      = 0x8b
	DeflateMethod        = 8
	GzipReservedMask     = 0xe0
	GzipHeaderCRCFlag    = 2
	GzipExtraFlag        = 4
	GzipFilenameFlag     = 8
	GzipCommentFlag      = 16
	GzipTrailerLength    = 8
	ZlibHeaderLength     = 2
	ZlibDictionaryFlag   = 0x20
	ZlibHeaderCheck      = 31
	ZlibMaxWindow        = 7
	ContentChecksumBytes = 4
	ContentLengthBytes   = 2
	ByteBits             = 8
	ZlibMethodBits       = 4
	ZlibMethodMask       = 0xf
)
