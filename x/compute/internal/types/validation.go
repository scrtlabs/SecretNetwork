package types

import (
	"net/url"
	"regexp"

	"cosmossdk.io/errors"
)

const (
	MaxWasmSize = 2 * 1024 * 1024 // 2MB

	// MaxLabelSize is the longest label that can be used when Instantiating a contract
	MaxLabelSize = 512

	// BuildTagRegexp is a docker image regexp.
	// We only support max 128 characters, with at least one organization name (subset of all legal names).
	//
	// Details from https://docs.docker.com/engine/reference/commandline/tag/#extended-description :
	//
	// An image name is made up of slash-separated name components (optionally prefixed by a registry hostname).
	// Name components may contain lowercase characters, digits and separators.
	// A separator is defined as a period, one or two underscores, or one or more dashes. A name component may not start or end with a separator.
	//
	// A tag name must be valid ASCII and may contain lowercase and uppercase letters, digits, underscores, periods and dashes.
	// A tag name may not start with a period or a dash and may contain a maximum of 128 characters.
	BuildTagRegexp = "^[a-z0-9][a-z0-9._-]*[a-z0-9](/[a-z0-9][a-z0-9._-]*[a-z0-9])+:[a-zA-Z0-9_][a-zA-Z0-9_.-]*$"

	MaxBuildTagSize = 128
)

func validateSourceURL(source string) error {
	if source != "" {
		u, err := url.Parse(source)
		if err != nil {
			return errors.Wrap(ErrInvalid, "not an url")
		}
		if !u.IsAbs() {
			return errors.Wrap(ErrInvalid, "not an absolute url")
		}
		if u.Scheme != "https" {
			return errors.Wrap(ErrInvalid, "must use https")
		}
	}
	return nil
}

func validateBuilder(buildTag string) error {
	if len(buildTag) > MaxBuildTagSize {
		return errors.Wrap(ErrLimit, "longer than 128 characters")
	}

	if buildTag != "" {
		ok, err := regexp.MatchString(BuildTagRegexp, buildTag)
		if err != nil || !ok {
			return ErrInvalid
		}
	}
	return nil
}

func validateWasmCode(s []byte) error {
	if len(s) == 0 {
		return errors.Wrap(ErrEmpty, "is required")
	}
	if len(s) > MaxWasmSize {
		return errors.Wrapf(ErrLimit, "cannot be longer than %d bytes", MaxWasmSize)
	}
	return nil
}

func validateLabel(label string) error {
	if label == "" {
		return errors.Wrap(ErrEmpty, "is required")
	}
	if len(label) > MaxLabelSize {
		return errors.Wrap(ErrLimit, "cannot be longer than 128 characters")
	}
	return nil
}
