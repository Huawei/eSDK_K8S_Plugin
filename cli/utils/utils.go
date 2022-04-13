package utils

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	versionMatchRE = regexp.MustCompile(`^\s*v?([0-9]+(?:\.[0-9x*]+)*)(.*)*$`)
	extraMatchRE   = regexp.MustCompile(`^(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?\s*$`)
)

type Version struct {
	components    []uint
	semver        bool
	datever       bool
	preRelease    string
	buildMetadata string
}

func MustParseSemantic(str string) *Version {
	v, err := ParseSemantic(str)
	if err != nil {
		panic(err.Error())
	}
	return v
}

func ParseSemantic(str string) (*Version, error) {
	return parse(str, true, false)
}

func parse(str string, semver bool, datever bool) (*Version, error) {
	parts := versionMatchRE.FindStringSubmatch(str)
	if parts == nil {
		return nil, fmt.Errorf("could not parse %q as version", str)
	}
	numbers, extra := parts[1], parts[2]

	components := strings.Split(numbers, ".")
	if ((semver || datever) && len(components) != 3) || (!semver && !datever && len(components) < 2) {
		return nil, fmt.Errorf("illegal version string %q", str)
	}

	v := &Version{
		components: make([]uint, len(components)),
		semver:     semver,
		datever:    datever,
	}
	for i, comp := range components {
		if i+1 == len(components) && (comp == "x" || comp == "*") {
			v.components[i] = math.MaxUint32
			continue
		}

		if (i == 0 || semver || (i != 1 && datever)) && strings.HasPrefix(comp, "0") && comp != "0" {
			return nil, fmt.Errorf("illegal zero-prefixed version component %q in %q", comp, str)
		}
		num, err := strconv.ParseUint(comp, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("illegal non-numeric version component %q in %q: %v", comp, str, err)
		}
		if i == 1 && datever && (num < 1 || num > 12) {
			return nil, fmt.Errorf("illegal month component %q in %q", comp, str)
		}
		v.components[i] = uint(num)
	}

	if (semver || datever) && extra != "" {
		extraParts := extraMatchRE.FindStringSubmatch(extra)
		if extraParts == nil {
			return nil, fmt.Errorf("could not parse pre-release/metadata (%s) in version %q", extra, str)
		}
		v.preRelease, v.buildMetadata = extraParts[1], extraParts[2]

		for _, comp := range strings.Split(v.preRelease, ".") {
			if _, err := strconv.ParseUint(comp, 10, 0); err == nil {
				if strings.HasPrefix(comp, "0") && comp != "0" {
					return nil, fmt.Errorf("illegal zero-prefixed version component %q in %q", comp, str)
				}
			}
		}
	}

	return v, nil
}

func (v *Version) LessThan(other *Version) bool {
	return v.compareInternal(other) == -1
}

func (v *Version) GreaterThan(other *Version) bool {
	for i := range v.components {
		if i+1 > len(other.components) || v.components[i] > other.components[i] {
			return true
		}
	}
	return false
}

func (v *Version) compareInternal(other *Version) int {
	for i := range v.components {
		switch {
		case i >= len(other.components):
			if v.components[i] != 0 {
				return 1
			}
		case other.components[i] < v.components[i]:
			return 1
		case other.components[i] > v.components[i]:
			return -1
		}
	}

	if !(v.semver || v.datever) || !(other.semver || other.datever) {
		return 0
	}

	switch {
	case v.preRelease == "" && other.preRelease != "":
		return 1
	case v.preRelease != "" && other.preRelease == "":
		return -1
	case v.preRelease == other.preRelease: // includes case where both are ""
		return 0
	}

	vPR := strings.Split(v.preRelease, ".")
	oPR := strings.Split(other.preRelease, ".")
	for i := range vPR {
		if i >= len(oPR) {
			return 1
		}
		vNum, err := strconv.ParseUint(vPR[i], 10, 0)
		if err == nil {
			oNum, err := strconv.ParseUint(oPR[i], 10, 0)
			if err == nil {
				switch {
				case oNum < vNum:
					return 1
				case oNum > vNum:
					return -1
				default:
					continue
				}
			}
		}
		if oPR[i] < vPR[i] {
			return 1
		} else if oPR[i] > vPR[i] {
			return -1
		}
	}

	return 0
}

func (v *Version) ShortString() string {
	var buffer bytes.Buffer

	for i, comp := range v.components {
		if i > 0 {
			buffer.WriteString(".")
		}
		if v.datever && i == 1 {
			buffer.WriteString(fmt.Sprintf("%02d", comp))
		} else {
			buffer.WriteString(fmt.Sprintf("%d", comp))
		}
	}

	return buffer.String()
}

func (v *Version) ToMajorMinorVersion() *Version {
	return MustParseGeneric(fmt.Sprintf("%d.%d", v.MajorVersion(), v.MinorVersion()))
}

func (v *Version) ToMajorMinorString() string {
	return fmt.Sprintf("%d.%d", v.MajorVersion(), v.MinorVersion())
}

func (v *Version) MajorVersion() uint {
	return v.components[0]
}

func (v *Version) MinorVersion() uint {
	return v.components[1]
}

func (v *Version) String() string {
	var buffer bytes.Buffer

	for i, comp := range v.components {
		if i > 0 {
			buffer.WriteString(".")
		}
		if v.datever && i == 1 {
			buffer.WriteString(fmt.Sprintf("%02d", comp))
		} else {
			buffer.WriteString(fmt.Sprintf("%d", comp))
		}
	}
	if v.preRelease != "" {
		buffer.WriteString("-")
		buffer.WriteString(v.preRelease)
	}
	if v.buildMetadata != "" {
		buffer.WriteString("+")
		buffer.WriteString(v.buildMetadata)
	}

	return buffer.String()
}

func MustParseGeneric(str string) *Version {
	v, err := ParseGeneric(str)
	if err != nil {
		panic(err)
	}
	return v
}

func ParseGeneric(str string) (*Version, error) {
	return parse(str, false, false)
}
