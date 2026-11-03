package cue

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	xunicode "golang.org/x/text/encoding/unicode"
)

// decodeCUEText preserves Unicode input and compares statistical evidence for
// legacy encodings. Successful decoding alone cannot distinguish GB18030/Big5.
func decodeCUEText(data []byte) (string, error) {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return validateCUEText(string(data[3:]))
	}
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) {
		return decodeCUEEncoding(data[2:], xunicode.UTF16(xunicode.LittleEndian, xunicode.IgnoreBOM))
	}
	if bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		return decodeCUEEncoding(data[2:], xunicode.UTF16(xunicode.BigEndian, xunicode.IgnoreBOM))
	}
	if utf8.Valid(data) {
		return validateCUEText(string(data))
	}

	results, err := chardet.NewTextDetector().DetectAll(data)
	if err != nil {
		return "", cueEncodingError("encoding could not be detected")
	}
	var bestText, bestCharset string
	var bestConfidence, runnerUp int
	for _, result := range results {
		var enc encoding.Encoding
		switch result.Charset {
		case "Big5": // x/text includes CP950 extensions.
			enc = traditionalchinese.Big5
		case "GB-18030": // Includes GBK and GB2312.
			enc = simplifiedchinese.GB18030
		case "Shift_JIS":
			enc = japanese.ShiftJIS
		case "EUC-KR":
			enc = korean.EUCKR
		default:
			continue
		}
		text, err := decodeCUEEncoding(data, enc)
		if err != nil {
			continue
		}
		if bestCharset == "" {
			bestText, bestCharset, bestConfidence = text, result.Charset, result.Confidence
		} else if text != bestText {
			runnerUp = result.Confidence
			break
		}
	}
	// ICU assigns 10 when a short sample merely has valid byte sequences.
	// Require statistical evidence and a clear lead over other valid decodings.
	if bestConfidence < 20 || bestConfidence-runnerUp < 10 {
		return "", cueEncodingError(fmt.Sprintf("uncertain encoding (best candidate %q, confidence %d, runner-up %d)", bestCharset, bestConfidence, runnerUp))
	}
	return bestText, nil
}

func decodeCUEEncoding(data []byte, enc encoding.Encoding) (string, error) {
	decoded, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", cueEncodingError("invalid encoded text")
	}
	// x/text decoders can replace invalid bytes with U+FFFD without an error.
	return validateCUEText(string(decoded))
}

func validateCUEText(text string) (string, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, utf8.RuneError) {
		return "", cueEncodingError("invalid or lossy text")
	}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			return "", cueEncodingError("unexpected control character in text")
		}
	}
	return text, nil
}

func cueEncodingError(reason string) error {
	return fmt.Errorf("%s; convert the CUE file to UTF-8 and reimport", reason)
}
