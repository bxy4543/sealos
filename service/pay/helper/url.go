package helper

import "strings"

func AppendURLParam(url, key, value string) string {
	if url == "" {
		return ""
	}
	if key == "" {
		return url
	}
	if value == "" {
		return url
	}
	if strings.Contains(url, "?") {
		url = url + "&" + key + "=" + value
	} else {
		url = url + "?" + key + "=" + value
	}
	return url
}
