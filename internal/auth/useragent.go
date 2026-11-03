package auth

import "strings"

// ParseUserAgent returns the browser and operating system named in a User-Agent
// header, as product names for the Devices list ("Firefox", "macOS"), or ""
// when unknown. The UI turns them into a label (research M4). Order matters:
// Edge and Chrome on iOS also claim to be Safari, iPhones claim to be Mac OS X,
// and Android claims to be Linux.
func ParseUserAgent(ua string) (browser, os string) {
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(ua, s) {
				return true
			}
		}
		return false
	}
	switch {
	case strings.HasPrefix(ua, "torq/"):
		browser = "torq"
	case has("Edg/", "EdgA/", "EdgiOS/"):
		browser = "Edge"
	case has("Firefox/", "FxiOS/"):
		browser = "Firefox"
	case has("Chrome/", "CriOS/"):
		browser = "Chrome"
	case has("Safari/"):
		browser = "Safari"
	}
	switch {
	case has("iPad"):
		os = "iPadOS"
	case has("iPhone", "iOS"):
		os = "iOS"
	case has("Android"):
		os = "Android"
	case has("Macintosh", "Mac OS X"):
		os = "macOS"
	case has("Windows"):
		os = "Windows"
	case has("Linux", "X11"):
		os = "Linux"
	}
	return browser, os
}
