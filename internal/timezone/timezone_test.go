// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package timezone // import "miniflux.app/v2/internal/timezone"

import (
	"testing"
	"time"

	// Make sure these tests pass when the timezone database is not installed on the host system.
	_ "time/tzdata"
)

func TestNow(t *testing.T) {
	tz := "Europe/Paris"
	now := Now(tz)

	if now.Location().String() != tz {
		t.Fatalf(`Unexpected timezone, got %q instead of %q`, now.Location(), tz)
	}
}

func TestNowWithInvalidTimezone(t *testing.T) {
	tz := "Invalid Timezone"
	expected := time.Local
	now := Now(tz)

	if now.Location().String() != expected.String() {
		t.Fatalf(`Unexpected timezone, got %q instead of %q`, now.Location(), expected)
	}
}

func TestConvertTimeWithNoTimezoneInformation(t *testing.T) {
	tz := "Canada/Pacific"
	input := time.Date(2018, 3, 1, 14, 2, 3, 0, time.FixedZone("", 0))
	output := Convert(tz, input)

	if output.Location().String() != tz {
		t.Fatalf(`Unexpected timezone, got %q instead of %s`, output.Location(), tz)
	}

	hours, minutes, secs := output.Clock()
	if hours != 14 || minutes != 2 || secs != 3 {
		t.Fatalf(`Unexpected time, got hours=%d, minutes=%d, secs=%d`, hours, minutes, secs)
	}
}

func TestConvertTimeWithDifferentTimezone(t *testing.T) {
	tz := "Canada/Central"
	input := time.Date(2018, 3, 1, 14, 2, 3, 0, time.UTC)
	output := Convert(tz, input)

	if output.Location().String() != tz {
		t.Fatalf(`Unexpected timezone, got %q instead of %s`, output.Location(), tz)
	}

	hours, minutes, secs := output.Clock()
	if hours != 8 || minutes != 2 || secs != 3 {
		t.Fatalf(`Unexpected time, got hours=%d, minutes=%d, secs=%d`, hours, minutes, secs)
	}
}

func TestConvertTimeWithIdenticalTimezone(t *testing.T) {
	tz := "Canada/Central"
	loc, _ := time.LoadLocation(tz)
	input := time.Date(2018, 3, 1, 14, 2, 3, 0, loc)
	output := Convert(tz, input)

	if output.Location().String() != tz {
		t.Fatalf(`Unexpected timezone, got %q instead of %s`, output.Location(), tz)
	}

	hours, minutes, secs := output.Clock()
	if hours != 14 || minutes != 2 || secs != 3 {
		t.Fatalf(`Unexpected time, got hours=%d, minutes=%d, secs=%d`, hours, minutes, secs)
	}
}

func TestConvertPostgresDateTimeWithNegativeTimezoneOffset(t *testing.T) {
	tz := "US/Eastern"
	input := time.Date(0, 1, 1, 0, 0, 0, 0, time.FixedZone("", -5))
	output := Convert(tz, input)

	if output.Location().String() != tz {
		t.Fatalf(`Unexpected timezone, got %q instead of %s`, output.Location(), tz)
	}

	if year := output.Year(); year != 0 {
		t.Fatalf(`Unexpected year, got %d instead of 0`, year)
	}
}
