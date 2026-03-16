package registry

import (
	"fmt"
	"regexp"

	"golang.org/x/mod/semver"
)

// semverFullRegexp matches a strict semver string with all three components
// (MAJOR.MINOR.PATCH), optional pre-release, and optional build metadata.
// It accepts an optional leading "v".
var semverFullRegexp = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

const (
	maxNameLen        = 64
	minNameLen        = 2
	maxDescriptionLen = 2000
	maxTagLen         = 64
	maxTags           = 20
	maxContentBytes   = 1024 * 1024 // 1MB
)

var nameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validateName(name string) error {
	if len(name) < minNameLen || len(name) > maxNameLen {
		return fmt.Errorf("name must be between %d and %d characters", minNameLen, maxNameLen)
	}
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("name must be lowercase alphanumeric with hyphens, cannot start or end with a hyphen")
	}
	return nil
}

func validateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version is required")
	}
	// Require all three components (MAJOR.MINOR.PATCH) before delegating to
	// semver.IsValid, which accepts partial versions like "v1.2".
	if !semverFullRegexp.MatchString(version) {
		return fmt.Errorf("version %q is not valid semver", version)
	}
	v := version
	if v[0] != 'v' {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return fmt.Errorf("version %q is not valid semver", version)
	}
	return nil
}

var tagRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validateTags(tags []string) error {
	if len(tags) > maxTags {
		return fmt.Errorf("too many tags (max %d)", maxTags)
	}
	for _, tag := range tags {
		if len(tag) > maxTagLen {
			return fmt.Errorf("tag %q exceeds max length of %d", tag, maxTagLen)
		}
		if len(tag) < 2 || !tagRegexp.MatchString(tag) {
			return fmt.Errorf("tag %q must be 2+ lowercase alphanumeric characters with hyphens", tag)
		}
	}
	return nil
}

func validatePublishRequest(name, version, description string, tags []string, content []byte) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateVersion(version); err != nil {
		return err
	}
	if description == "" {
		return fmt.Errorf("description is required")
	}
	if len(description) > maxDescriptionLen {
		return fmt.Errorf("description exceeds max length of %d", maxDescriptionLen)
	}
	if err := validateTags(tags); err != nil {
		return err
	}
	if len(content) == 0 {
		return fmt.Errorf("content is required")
	}
	if len(content) > maxContentBytes {
		return fmt.Errorf("content exceeds max size of %d bytes", maxContentBytes)
	}
	return nil
}
