package photomaterialize

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	pythonScientificLarge = 1e16
	pythonScientificSmall = 1e-4
)

// OwnedXMP reports whether a complete XML document belongs to Source's toolkit.
func OwnedXMP(data []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	state := new(xmpOwnership)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return state.owned && state.seen
		}

		if err != nil || !state.accept(token) {
			return false
		}
	}
}

type xmpOwnership struct {
	depth int
	seen  bool
	owned bool
}

func (state *xmpOwnership) accept(token xml.Token) bool {
	switch value := token.(type) {
	case xml.EndElement:
		state.depth--
	case xml.StartElement:
		return state.start(value)
	case xml.CharData:
		return state.depth != 0 || strings.TrimSpace(string(value)) == ""
	case xml.ProcInst, xml.Comment, xml.Directive:
		return true
	}

	return true
}

func (state *xmpOwnership) start(value xml.StartElement) bool {
	if state.seen && state.depth == 0 {
		return false
	}

	state.depth++
	if state.seen {
		return true
	}

	state.seen = true

	for _, attribute := range value.Attr {
		if attribute.Name.Local == "xmptk" && attribute.Name.Space == "adobe:ns:meta/" {
			state.owned = strings.HasPrefix(attribute.Value, string(SourceToolkit))
		}
	}

	return true
}

// RenderXMP renders the six optional metadata groups in Source order.
func RenderXMP(metadata Metadata) []byte {
	var output strings.Builder

	output.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
	output.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="`)
	output.WriteString(escapeAttribute(metadata.Toolkit))
	output.WriteString(`"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`)
	writeDescription(&output, "dc", "http://purl.org/dc/elements/1.1/", dcFields(metadata))
	writeDescription(&output, "exif", "http://ns.adobe.com/exif/1.0/", exifFields(metadata))
	writeDescription(&output, "Iptc4xmpExt", "http://iptc.org/std/Iptc4xmpExt/2008-02-29/",
		stringElement("Iptc4xmpExt:DigitalSourceType", metadata.DigitalSourceType))
	writeDescription(&output, "photoshop", "http://ns.adobe.com/photoshop/1.0/",
		dateElement("photoshop:DateCreated", metadata.CreateDate))
	writeDescription(&output, "tiff", "http://ns.adobe.com/tiff/1.0/",
		integerElement("tiff:Orientation", metadata.Orientation)+stringElement("tiff:Make", metadata.Make))
	writeDescription(&output, "xmp", "http://ns.adobe.com/xap/1.0/",
		dateElement("xmp:CreateDate", metadata.CreateDate)+integerElement("xmp:Rating", metadata.Rating))
	output.WriteString("</rdf:RDF></x:xmpmeta>")

	return []byte(strings.ReplaceAll(output.String(),
		`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"></rdf:RDF>`,
		`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" />`))
}

func writeDescription(output *strings.Builder, prefix, namespace, fields string) {
	if fields == "" {
		return
	}

	output.WriteString(`<rdf:Description rdf:about="" xmlns:` + prefix + `="` + namespace + `">`)
	output.WriteString(fields)
	output.WriteString("</rdf:Description>")
}

func dcFields(metadata Metadata) string {
	fields := stringElement("dc:title", metadata.Title) + stringElement("dc:description", metadata.Description)
	if metadata.Keywords == nil || len(*metadata.Keywords) == 0 {
		return fields
	}

	var output strings.Builder

	output.WriteString(fields)
	output.WriteString("<dc:subject><rdf:Seq>")

	for _, keyword := range *metadata.Keywords {
		output.WriteString(element("rdf:li", keyword))
	}

	output.WriteString("</rdf:Seq></dc:subject>")

	return output.String()
}

func exifFields(metadata Metadata) string {
	return floatElement("exif:GPSAltitude", metadata.GpsAltitude) +
		floatElement("exif:GPSLatitude", metadata.GpsLatitude) +
		floatElement("exif:GPSLongitude", metadata.GpsLongitude) +
		floatElement("exif:GPSSpeed", metadata.GpsSpeed) +
		gpsDateElement(metadata.GpsTimestamp)
}

func gpsDateElement(value *time.Time) string {
	if value == nil {
		return ""
	}

	// Plist timestamps are naive UTC datetimes in Source.
	return element("exif:GPSTimeStamp", value.Format("2006-01-02T15:04:05"))
}

func stringElement(name string, value *string) string {
	if value == nil || *value == "" {
		return ""
	}

	return element(name, *value)
}

func integerElement(name string, value *int) string {
	if value == nil {
		return ""
	}

	return element(name, strconv.Itoa(*value))
}

func floatElement(name string, value *float64) string {
	if value == nil {
		return ""
	}

	return element(name, pythonFloat(*value))
}

func pythonFloat(value float64) string {
	format := byte('f')
	if absolute := math.Abs(value); absolute >= pythonScientificLarge ||
		absolute != 0 && absolute < pythonScientificSmall {
		format = 'e'
	}

	text := strconv.FormatFloat(value, format, -1, 64)
	text = strings.ReplaceAll(text, "NaN", "nan")
	text = strings.ReplaceAll(text, "+Inf", "inf")

	text = strings.ReplaceAll(text, "-Inf", "-inf")

	if strings.Contains(text, "nan") || strings.Contains(text, "inf") {
		return text
	}

	if !strings.ContainsAny(text, ".e") {
		text += ".0"
	}

	return text
}

func dateElement(name string, value *time.Time) string {
	if value == nil {
		return ""
	}

	return element(name, value.Format("2006-01-02T15:04:05-0700"))
}

func element(name, text string) string {
	if text == "" {
		return "<" + name + " />"
	}

	return "<" + name + ">" + escapeText(text) + "</" + name + ">"
}

func escapeText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func escapeAttribute(value string) string {
	return strings.ReplaceAll(escapeText(value), `"`, "&quot;")
}
