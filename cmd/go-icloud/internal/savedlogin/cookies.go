package savedlogin

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/referenceconfig"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errCookieFormat = errors.New("unsupported reference cookie file")

func readCookies(path string) ([]icloud.AuthCookie, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read reference cookies: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	if !scanner.Scan() || scanner.Text() != string(referenceconfig.LWPCookies2) {
		return nil, errCookieFormat
	}

	cookies := []icloud.AuthCookie{}

	for scanner.Scan() {
		text := scanner.Text()
		if !strings.HasPrefix(text, string(referenceconfig.CookieRecord)) {
			continue
		}

		values, parseErr := parseCookieLine(text)
		if parseErr != nil {
			return nil, parseErr
		}

		cookies = append(cookies, values...)
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("scan reference cookies: %w", err)
	}

	return cookies, nil
}

type cookieWord struct {
	name     string
	value    string
	assigned bool
}

func cookieWords(text string) ([][]cookieWord, error) {
	groups := [][]cookieWord{}
	words := []cookieWord{}

	for len(text) != 0 {
		text = strings.TrimLeft(text, " ;\t\r\n")
		if text == "" {
			break
		}

		if text[0] == ',' {
			groups = append(groups, words)
			words = []cookieWord{}
			text = text[1:]

			continue
		}

		end := strings.IndexAny(text, "=;, \t")
		if end < 0 {
			end = len(text)
		}

		if end == 0 {
			return nil, errCookieFormat
		}

		word := cookieWord{name: text[:end], value: "", assigned: false}

		text = strings.TrimLeft(text[end:], " \t")

		if strings.HasPrefix(text, "=") {
			word.assigned = true

			var err error

			word.value, text, err = cookieValue(strings.TrimLeft(text[1:], " \t"))
			if err != nil {
				return nil, err
			}
		}

		words = append(words, word)
	}

	if len(words) != 0 {
		groups = append(groups, words)
	}

	return groups, nil
}

func cookieValue(text string) (string, string, error) {
	if !strings.HasPrefix(text, "\"") {
		end := strings.IndexAny(text, ";,")
		if end < 0 {
			end = len(text)
		}

		return strings.TrimSpace(text[:end]), text[end:], nil
	}

	var value strings.Builder

	for index := 1; index < len(text); index++ {
		character := text[index]
		if character == '"' {
			return value.String(), text[index+1:], nil
		}

		if character == '\\' {
			index++
			if index == len(text) {
				return "", "", errCookieFormat
			}

			character = text[index]
		}

		value.WriteByte(character)
	}

	return "", "", errCookieFormat
}

func importedCookie(words []cookieWord) (icloud.AuthCookie, bool, error) {
	var cookie icloud.AuthCookie
	if len(words) == 0 || !words[0].assigned {
		return cookie, false, errCookieFormat
	}

	attributes := map[string]cookieWord{}
	for _, word := range words[1:] {
		attributes[strings.ToLower(word.name)] = word
	}

	if attributes["version"].value != "0" || attributes["port"].value != "" || flagCookie(attributes, "port_spec") {
		return cookie, false, errCookieFormat
	}

	cookie.Name, cookie.Value = words[0].name, words[0].value
	cookie.Domain, cookie.Path = attributes["domain"].value, attributes["path"].value
	cookie.HostOnly = !strings.HasPrefix(cookie.Domain, ".")
	cookie.Secure = flagCookie(attributes, "secure")

	_, cookie.HTTPOnly = attributes["httponly"]

	if expires := attributes["expires"].value; expires != "" {
		value, err := time.Parse("2006-01-02 15:04:05Z", expires)
		if err != nil {
			return cookie, false, fmt.Errorf("decode reference cookie expiry: %w", err)
		}

		cookie.Expires = &value
		if !value.After(time.Now()) {
			return cookie, false, nil
		}
	}

	return cookie, cookie.Name != string(referenceconfig.DiscardFindMyCookie), nil
}

func flagCookie(attributes map[string]cookieWord, name string) bool {
	value, exists := attributes[name]

	return exists && (!value.assigned || value.value != "")
}

func parseCookieLine(text string) ([]icloud.AuthCookie, error) {
	groups, err := cookieWords(strings.TrimSpace(strings.TrimPrefix(text, string(referenceconfig.CookieRecord))))
	if err != nil {
		return nil, err
	}

	cookies := []icloud.AuthCookie{}

	for _, words := range groups {
		cookie, keep, parseErr := importedCookie(words)
		if parseErr != nil {
			return nil, parseErr
		}

		if keep {
			cookies = append(cookies, cookie)
		}
	}

	return cookies, nil
}
