package idgen

import "fmt"

const hgBase62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func hgEncodeBase62(value uint64) string {
	if value == 0 {
		return "0"
	}

	var buffer [11]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = hgBase62Alphabet[value%62]
		value /= 62
	}

	return string(buffer[index:])
}

func hgDecodeBase62(encoded string) (uint64, error) {
	if encoded == "" || (len(encoded) > 1 && encoded[0] == '0') {
		return 0, ErrHGInvalidID
	}

	var value uint64
	for index := 0; index < len(encoded); index++ {
		digit, ok := hgBase62Digit(encoded[index])
		if !ok {
			return 0, fmt.Errorf("%w: invalid base62 character at index %d", ErrHGInvalidID, index)
		}
		if value > (^uint64(0)-digit)/62 {
			return 0, fmt.Errorf("%w: base62 value overflows uint64", ErrHGInvalidID)
		}
		value = value*62 + digit
	}

	return value, nil
}

func hgBase62Digit(character byte) (uint64, bool) {
	switch {
	case character >= '0' && character <= '9':
		return uint64(character - '0'), true
	case character >= 'A' && character <= 'Z':
		return uint64(character-'A') + 10, true
	case character >= 'a' && character <= 'z':
		return uint64(character-'a') + 36, true
	default:
		return 0, false
	}
}
