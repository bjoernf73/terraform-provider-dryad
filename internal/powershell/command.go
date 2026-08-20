package powershell

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"
	"unicode/utf16"
)

func BuildCommand(powerShellPath string, script string) (string, error) {
	if strings.TrimSpace(powerShellPath) == "" {
		return "", fmt.Errorf("powershell_path must not be empty")
	}

	encoded := encodeUTF16LEBase64(script)
	return fmt.Sprintf(`"%s" -NoLogo -NoProfile -NonInteractive -EncodedCommand %s`, powerShellPath, encoded), nil
}

func encodeUTF16LEBase64(input string) string {
	utf16Data := utf16.Encode([]rune(input))
	bytes := make([]byte, 0, len(utf16Data)*2)

	for _, codeUnit := range utf16Data {
		bytes = append(bytes, byte(codeUnit), byte(codeUnit>>8))
	}

	return base64.StdEncoding.EncodeToString(bytes)
}

type clixmlMessage struct {
	Strings []clixmlString `xml:"S"`
}

type clixmlString string

func (s *clixmlString) UnmarshalText(text []byte) error {
	value := strings.TrimSpace(string(text))
	if strings.HasPrefix(value, "+ ") {
		value = "\n" + strings.TrimPrefix(value, "+ ")
	}
	*s = clixmlString(value)
	return nil
}

func DecodeCLIXML(input string) string {
	if !strings.Contains(input, "#< CLIXML") {
		return input
	}

	xmlInput := strings.ReplaceAll(input, "#< CLIXML", "")
	var message clixmlMessage

	if err := xml.Unmarshal([]byte(xmlInput), &message); err != nil {
		return input
	}

	parts := make([]string, 0, len(message.Strings))
	for _, item := range message.Strings {
		parts = append(parts, string(item))
	}

	joined := strings.Join(parts, "")
	replacer := strings.NewReplacer("_x000D_", "", "_x000A_", "")
	return strings.TrimSpace(replacer.Replace(joined))
}
