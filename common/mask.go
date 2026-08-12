package common

import "strings"

// MaskPhone keeps the first 3 and last 4 characters, with at least 4 stars in
// between. Short numbers (length 3-7) become first char + stars + last char;
// very short (<= 2) numbers are fully masked. Empty strings pass through.
func MaskPhone(phone string) string {
	n := len(phone)
	if n == 0 {
		return ""
	}
	if n <= 2 {
		return strings.Repeat("*", n)
	}
	if n <= 7 {
		return string(phone[0]) + strings.Repeat("*", n-2) + string(phone[n-1])
	}
	return phone[:3] + strings.Repeat("*", 4) + phone[n-4:]
}
