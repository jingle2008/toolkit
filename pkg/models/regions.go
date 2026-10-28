package models

import (
	"fmt"
	"maps"
	"strings"
)

// Region is an OCI region identifier, e.g. "us-phoenix-1".
type Region string

// The regions the code names itself.
const (
	regionIAD Region = "us-ashburn-1"
	regionPHX Region = "us-phoenix-1"
)

//go:generate go run ../../internal/tools/genregions -o regions_sdk.go

// extraShortNames are short codes for regions the OCI SDK doesn't list:
// sovereign and dedicated-cloud regions.
var extraShortNames = map[string]Region{
	"dca": "us-gov-sterling-2",
	"ftw": "us-gov-fortworth-1",
	"fyv": "ap-kulai-1",
	"gcn": "us-dcc-phoenix-2",
	"ifp": "us-dcc-phoenix-1",
	"tiw": "us-tacoma-1",
	"tyo": "ap-dcc-tokyo-1",
}

// regionByShortName maps each short code to its region: the SDK's
// (sdkShortNames, generated) and extraShortNames.
var regionByShortName = func() map[string]Region {
	m := maps.Clone(sdkShortNames)
	maps.Copy(m, extraShortNames)
	return m
}()

// shortNameByRegion is regionByShortName reversed.
var shortNameByRegion = func() map[Region]string {
	m := make(map[Region]string, len(regionByShortName))
	for k, v := range regionByShortName {
		m[v] = k
	}
	return m
}()

/*
Code returns the short code for the Region.

For regions that haven't been added to the regionByShortName map yet
(typically newer SDK additions or sovereign-cloud regions that never
ship in the public SDK), it falls back to the second-to-last
dash-delimited segment of the region identifier — e.g.

	"ap-westtokyo-1" -> "westtokyo"
	"us-newark-1"    -> "pgc" (mapped)
	"unknown-region" -> "UNKNOWN" (segment count < 3)

The fallback is informational; callers that need the canonical OCI
region key should add the mapping explicitly. Returns the literal
"UNKNOWN" for inputs that don't look like region identifiers at all.
*/
func (r Region) Code() string {
	if k, ok := shortNameByRegion[r]; ok {
		return k
	}
	// Fallback: derive a slug from the region name so the table shows
	// something identifiable rather than literal "UNKNOWN" for regions
	// that haven't been mapped yet.
	if parts := strings.Split(string(r), "-"); len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	return "UNKNOWN"
}

/*
ResolveRegion canonicalizes a user-supplied region into a full OCI
region identifier.

The input is lowercased and trimmed first. A value containing a dash
is taken to be a full identifier already and returned unchanged;
a dash-free value is looked up as a short code, so "phx" resolves to
"us-phoenix-1". The dash is a sound discriminator against this data:
every mapped region has at least two, and no short code has any.

An unrecognized dash-free value is an error rather than a
pass-through. It can only be a mistyped code, and saying so here beats
the loader's generic "environment is not valid or in the list", which
reads as though the realm or type were at fault.

Callers must resolve before building an Environment: the region string
reaches the OCI SDK and OCID construction verbatim, so a short code
surviving that far yields a wrong endpoint and wrong OCIDs.
*/
func ResolveRegion(v string) (Region, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if strings.Contains(v, "-") {
		return Region(v), nil
	}
	if r, ok := regionByShortName[v]; ok {
		return r, nil
	}
	return "", fmt.Errorf(
		"unknown region code %q (want a short code like \"phx\" or a full identifier like \"us-phoenix-1\")", v,
	)
}
