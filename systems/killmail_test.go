package systems

import (
	"testing"

	"git.sr.ht/~barveyhirdman/chainkills/config"
	"github.com/stretchr/testify/require"
)

// var cfg = config.Cfg{
// 	Friends: config.Friends{
// 		Alliances: []uint64{
// 			1,
// 		},
// 		Corporations: []uint64{
// 			2,
// 		},
// 		Characters: []uint64{
// 			3,
// 		},
// 	},
// }

func TestCharacterIsFriend(t *testing.T) {
	require.NoError(t, config.Read("testdata/config.test.yaml"))

	tests := []struct {
		label     string
		character CharacterInfo
		expected  bool
	}{
		{
			label:     "empty character",
			character: CharacterInfo{},
			expected:  false,
		},
		{
			label:     "alliance match",
			character: CharacterInfo{AllianceID: 1},
			expected:  true,
		},
		{
			label:     "corp match",
			character: CharacterInfo{CorporationID: 2},
			expected:  true,
		},
		{
			label:     "character match",
			character: CharacterInfo{CharacterID: 3},
			expected:  true,
		},
		{
			label:     "none match",
			character: CharacterInfo{},
			expected:  false,
		},
	}

	for _, tt := range tests {
		tf := func(t *testing.T) {
			require.Equal(t, tt.expected, tt.character.IsFriend())
		}

		t.Run(tt.label, tf)
	}
}

func TestKillmailColor(t *testing.T) {
	require.NoError(t, config.Read("testdata/config.test.yaml"))

	tests := []struct {
		label         string
		killmail      Killmail
		expectedColor int
	}{
		{
			label: "neutral",
			killmail: Killmail{
				Attackers: []CharacterInfo{},
				Victim:    CharacterInfo{},
			},
			expectedColor: ColorWhatever,
		},
		{
			label: "attacker alliance match",
			killmail: Killmail{
				Attackers: []CharacterInfo{
					{AllianceID: 1},
				},
				Victim: CharacterInfo{},
			},
			expectedColor: ColorOurKill,
		},
		{
			label: "attacker alliance match",
			killmail: Killmail{
				Attackers: []CharacterInfo{
					{CorporationID: 2},
				},
				Victim: CharacterInfo{},
			},
			expectedColor: ColorOurKill,
		},
		{
			label: "attacker alliance match",
			killmail: Killmail{
				Attackers: []CharacterInfo{
					{CharacterID: 3},
				},
				Victim: CharacterInfo{},
			},
			expectedColor: ColorOurKill,
		},
		{
			label: "victim alliance match",
			killmail: Killmail{
				Attackers: []CharacterInfo{},
				Victim: CharacterInfo{
					AllianceID: 1,
				},
			},
			expectedColor: ColorOurLoss,
		},
		{
			label: "victim corp match",
			killmail: Killmail{
				Attackers: []CharacterInfo{},
				Victim: CharacterInfo{
					CorporationID: 2,
				},
			},
			expectedColor: ColorOurLoss,
		},
		{
			label: "victim character match",
			killmail: Killmail{
				Attackers: []CharacterInfo{},
				Victim: CharacterInfo{
					CharacterID: 3,
				},
			},
			expectedColor: ColorOurLoss,
		},
	}

	for _, tt := range tests {
		tf := func(t *testing.T) {
			require.Equal(t, tt.expectedColor, tt.killmail.Color())
		}

		t.Run(tt.label, tf)
	}
}
