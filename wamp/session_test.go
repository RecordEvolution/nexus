package wamp //nolint:testpackage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsNewRecvIDBounds(t *testing.T) {
	testCases := [...]struct {
		last     ID
		id       ID
		expected bool
	}{
		{last: 0, id: 1, expected: true},
		{last: 1, id: 0, expected: false},
		{last: 1, id: 1, expected: false},
		{last: ID(MaxID), id: 1, expected: true},        // rollover
		{last: MaxID - 250, id: 1, expected: true},      // rollover w/ fudge
		{last: MaxID - deltaID, id: 1, expected: false}, // not yet a rollover
		// valid for rollover but new value is too far out of bounds
		{last: MaxID - deltaID + 100, id: deltaID + 100, expected: false},
		{last: MaxID - deltaID + 1, id: 1, expected: false},
		{last: 2, id: 1, expected: false},
		// just within bounds after rollover
		{last: MaxID - deltaID + 2, id: 1, expected: true},
		// id is technically out of bounds but that's OK
		{last: ID(MaxID), id: deltaID - 1, expected: true},
		// Jump forward any amount.
		{last: 1, id: MaxID - deltaID + 1, expected: true},
		{last: 100, id: 100 + deltaID, expected: true},
		// illegal id values
		{last: MaxID, id: MaxID + 1, expected: false},
		{last: MaxID, id: 0, expected: false},
	}

	s := Session{lastRecvID: 0}
	for i, tc := range testCases {
		name := fmt.Sprintf("%02d_%d-%d-%v", i, tc.last, tc.id, tc.expected)
		t.Run(name, func(t *testing.T) {
			s.lastRecvID = tc.last
			require.Equal(t, tc.expected, s.IsNewRecvID(tc.id))
		})
	}
}

func TestRolesDictRoundTrips(t *testing.T) {
	greet := Dict{"roles": Dict{
		"publisher":  Dict{"features": Dict{"publisher_exclusion": true, "off_feature": false}},
		"subscriber": Dict{},
	}}
	s := NewSession(nil, 1, nil, greet)
	clone := NewSession(nil, 2, nil, Dict{"roles": s.RolesDict()})
	if !clone.HasRole("publisher") || !clone.HasRole("subscriber") {
		t.Fatalf("roles lost in round trip: %v", s.RolesDict())
	}
	if !clone.HasFeature("publisher", "publisher_exclusion") {
		t.Error("feature lost in round trip")
	}
	if clone.HasFeature("publisher", "off_feature") {
		t.Error("disabled feature resurrected")
	}
	if NewSession(nil, 3, nil, nil).RolesDict() != nil {
		t.Error("no roles must return nil")
	}
}
