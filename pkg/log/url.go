package log

import (
	"log/slog"
	"net/url"
)

func ScrubbedURL(name string, rawURL string) slog.Attr {
	u, err := url.Parse(rawURL)
	if err != nil {
		return slog.String(name, rawURL)
	}

	var str string
	if u.User != nil {
		copy := u.JoinPath()
		copy.User = url.UserPassword("xxx", "xxx")
		str = copy.String()
	} else {
		str = u.String()
	}

	return slog.String(name, str)
}
