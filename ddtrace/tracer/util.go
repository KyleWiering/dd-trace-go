// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016 Datadog, Inc.

package tracer

import (
	"fmt"
	"strconv"
	"strings"
)

// parseUint64 parses a uint64 from either an unsigned 64 bit base-10 string
// or a signed 64 bit base-10 string representing an unsigned integer
func parseUint64(str string) (uint64, error) {
	if strings.HasPrefix(str, "-") {
		id, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint64(id), nil
	}
	return strconv.ParseUint(str, 10, 64)
}

func isValidPropagatableTag(k, v string) error {
	if len(k) == 0 {
		return fmt.Errorf("key length must be greater than zero")
	}
	for _, ch := range k {
		if ch < 32 || ch > 126 || ch == ' ' || ch == '=' || ch == ',' {
			return fmt.Errorf("key contains an invalid character %d", ch)
		}
	}
	if len(v) == 0 {
		return fmt.Errorf("value length must be greater than zero")
	}
	for _, ch := range v {
		if ch < 32 || ch > 126 || ch == ',' {
			return fmt.Errorf("value contains an invalid character %d", ch)
		}
	}
	return nil
}

func parsePropagatableTraceTags(s string) (map[string]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	tags := make(map[string]string)
	searchingKey, start := true, 0
	var key string
	for i, ch := range s {
		switch ch {
		case '=':
			if searchingKey {
				if i-start == 0 {
					return nil, fmt.Errorf("invalid format")
				}
				key = s[start:i]
				searchingKey, start = false, i+1
			}
		case ',':
			if searchingKey || i-start == 0 {
				return nil, fmt.Errorf("invalid format")
			}
			tags[key] = s[start:i]
			searchingKey, start = true, i+1
		}
	}
	if searchingKey || len(s)-start == 0 {
		return nil, fmt.Errorf("invalid format")
	}
	tags[key] = s[start:]
	return tags, nil
}
