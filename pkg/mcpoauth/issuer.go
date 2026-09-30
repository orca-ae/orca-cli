// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// compatibleIssuer deliberately relaxes RFC 8414 issuer equality for HTTPS
// gateways delegating to an issuer in the same registrable domain. Private PSL
// suffixes isolate hosted tenants; this is not proof of common administration.
// IPs, local/unknown suffixes and different ports get no implicit exception.
func compatibleIssuer(expected, actual string) bool {
	if expected == actual {
		return true
	}
	a, err := url.Parse(expected)
	if err != nil {
		return false
	}
	b, err := url.Parse(actual)
	if err != nil || a.Scheme != "https" || b.Scheme != "https" {
		return false
	}
	port := func(u *url.URL) string {
		if u.Port() == "" {
			return "443"
		}
		return u.Port()
	}
	if port(a) != port(b) {
		return false
	}
	domain := registrableDomain(a.Hostname())
	return domain != "" && domain == registrableDomain(b.Hostname())
}

func registrableDomain(host string) string {
	host, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return ""
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if net.ParseIP(host) != nil {
		return ""
	}
	suffix, icann := publicsuffix.PublicSuffix(host)
	// PSL falls back to the last label for unlisted suffixes. Known private
	// suffixes (e.g. github.io) are multi-label and must not be discarded.
	if !icann && !strings.Contains(suffix, ".") {
		return ""
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}
	return domain
}
