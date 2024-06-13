package client_test

import (
	"net/netip"
	"testing"

	"github.com/AdguardTeam/AdGuardHome/internal/client"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/stretchr/testify/require"
)

func TestStorage_Add(t *testing.T) {
	testCases := []struct {
		name       string
		cli        *client.Persistent
		wantErrMsg string
	}{{
		name: "basic",
		cli: &client.Persistent{
			Name: "basic",
			IPs: []netip.Addr{
				netip.MustParseAddr("1.2.3.4"),
			},
			UID: client.MustNewUID(),
		},
		wantErrMsg: "",
	}, {
		name:       "nil",
		cli:        nil,
		wantErrMsg: "adding client: client is nil",
	}, {
		name: "empty_name",
		cli: &client.Persistent{
			Name: "",
		},
		wantErrMsg: "adding client: empty name",
	}, {
		name: "no_id",
		cli: &client.Persistent{
			Name: "no_id",
		},
		wantErrMsg: "adding client: id required",
	}, {
		name: "no_uid",
		cli: &client.Persistent{
			Name: "no_uid",
			IPs: []netip.Addr{
				netip.MustParseAddr("1.2.3.4"),
			},
		},
		wantErrMsg: "adding client: uid required",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := client.NewStorage(nil)
			err := s.Add(tc.cli)

			testutil.AssertErrorMsg(t, tc.wantErrMsg, err)
		})
	}

	t.Run("duplicate_uid", func(t *testing.T) {
		sameUID := client.MustNewUID()
		s := client.NewStorage(nil)

		cli1 := &client.Persistent{
			Name: "cli1",
			IPs:  []netip.Addr{netip.MustParseAddr("1.2.3.4")},
			UID:  sameUID,
		}

		cli2 := &client.Persistent{
			Name: "cli2",
			IPs:  []netip.Addr{netip.MustParseAddr("4.3.2.1")},
			UID:  sameUID,
		}

		err := s.Add(cli1)
		require.NoError(t, err)

		err = s.Add(cli2)
		testutil.AssertErrorMsg(t, `adding client: another client "cli1" uses the same uid`, err)
	})
}
