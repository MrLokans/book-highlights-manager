package auth

import (
	"html/template"
	"net/url"
)

// AssetURL returns a /static URL tagged with the app version, so a CDN or browser
// that caches static files fetches fresh copies after every release.
func AssetURL(version, path string) string {
	return "/static/" + path + "?v=" + url.QueryEscape(version)
}

// AssetFuncs exposes AssetURL to templates as {{ asset "style.css" }}.
func AssetFuncs(version string) template.FuncMap {
	return template.FuncMap{"asset": func(path string) string { return AssetURL(version, path) }}
}
