package orchestrator

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/tmux"
)

func TestIsSystemTeam(t *testing.T) {
	tests := []struct {
		name   string
		teamID string
		want   bool
	}{
		{"unaffiliated team ID", UnaffiliatedTeamID, true},
		{"regular UUID", "550e8400-e29b-41d4-a716-446655440000", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSystemTeam(tt.teamID); got != tt.want {
				t.Fatalf("IsSystemTeam(%q) = %v, want %v", tt.teamID, got, tt.want)
			}
		})
	}
}

func TestEnsureUnaffiliatedTeam_CreatesOnFirstCall(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	team, err := s.EnsureUnaffiliatedTeam("global", "")
	if err != nil {
		t.Fatalf("EnsureUnaffiliatedTeam() error = %v", err)
	}
	if team.ID != UnaffiliatedTeamID {
		t.Fatalf("team.ID = %q, want %q", team.ID, UnaffiliatedTeamID)
	}
	if team.Name != UnaffiliatedTeamName {
		t.Fatalf("team.Name = %q, want %q", team.Name, UnaffiliatedTeamName)
	}
	if team.StorageLocation != "global" {
		t.Fatalf("team.StorageLocation = %q, want %q", team.StorageLocation, "global")
	}
	if len(team.Members) != 0 {
		t.Fatalf("len(team.Members) = %d, want 0", len(team.Members))
	}
}

func TestEnsureUnaffiliatedTeam_ReturnsExistingOnSecondCall(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	team1, err := s.EnsureUnaffiliatedTeam("global", "")
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}

	team2, err := s.EnsureUnaffiliatedTeam("global", "")
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}

	if team1.ID != team2.ID {
		t.Fatalf("team IDs differ: %q != %q", team1.ID, team2.ID)
	}
}

func TestAddMemberToUnaffiliatedTeam(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	member := TeamMember{
		PaneTitle: "TestMember",
		Role:      "Tester",
		Command:   "claude",
	}
	err := s.AddMemberToUnaffiliatedTeam(member, "global", "")
	if err != nil {
		t.Fatalf("AddMemberToUnaffiliatedTeam() error = %v", err)
	}

	// Verify the member was saved by loading the unaffiliated team.
	team, err := s.EnsureUnaffiliatedTeam("global", "")
	if err != nil {
		t.Fatalf("EnsureUnaffiliatedTeam() error = %v", err)
	}
	if len(team.Members) != 1 {
		t.Fatalf("len(team.Members) = %d, want 1", len(team.Members))
	}
	if team.Members[0].PaneTitle != "TestMember" {
		t.Fatalf("member.PaneTitle = %q, want %q", team.Members[0].PaneTitle, "TestMember")
	}
	if team.Members[0].TeamID != UnaffiliatedTeamID {
		t.Fatalf("member.TeamID = %q, want %q", team.Members[0].TeamID, UnaffiliatedTeamID)
	}
}

func TestAddMemberToUnaffiliatedTeam_MultipleMembersIncrementOrder(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	for i, title := range []string{"First", "Second", "Third"} {
		err := s.AddMemberToUnaffiliatedTeam(TeamMember{
			PaneTitle: title,
			Role:      "Role",
			Command:   "cmd",
		}, "global", "")
		if err != nil {
			t.Fatalf("AddMemberToUnaffiliatedTeam(%d) error = %v", i, err)
		}
	}

	team, err := s.EnsureUnaffiliatedTeam("global", "")
	if err != nil {
		t.Fatalf("EnsureUnaffiliatedTeam() error = %v", err)
	}
	if len(team.Members) != 3 {
		t.Fatalf("len(team.Members) = %d, want 3", len(team.Members))
	}
	// Members should be ordered 0, 1, 2.
	for i, m := range team.Members {
		if m.Order != i {
			t.Fatalf("member[%d].Order = %d, want %d", i, m.Order, i)
		}
	}
}

func TestAddMemberToUnaffiliatedTeam_InvalidMember(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	err := s.AddMemberToUnaffiliatedTeam(TeamMember{}, "global", "")
	if err == nil {
		t.Fatal("expected error for empty member")
	}
	if !strings.Contains(err.Error(), "member validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddMemberToUnaffiliatedTeam_DoesNotMutateInput(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	member := TeamMember{
		PaneTitle: "  Lead  ",
		Role:      "  Tester  ",
		Command:   "  claude  ",
		Args:      []string{"  --verbose  ", "   "},
	}
	original := member
	original.Args = append([]string(nil), member.Args...)

	if err := s.AddMemberToUnaffiliatedTeam(member, "global", ""); err != nil {
		t.Fatalf("AddMemberToUnaffiliatedTeam() error = %v", err)
	}

	if !reflect.DeepEqual(member, original) {
		t.Fatalf("input member mutated: got %#v want %#v", member, original)
	}
}

func TestStartTeam_RejectsSystemTeam(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := testDeps(t, configPath)
	deps.FindSessionSnapshot = func(string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{Name: "test-session", RootPath: "/test"}, nil
	}
	deps.GetActiveSessionName = func() string { return "test-session" }
	s := NewService(deps)

	// Create the unaffiliated team with a member.
	err := s.AddMemberToUnaffiliatedTeam(TeamMember{
		PaneTitle: "Lead",
		Role:      "Lead",
		Command:   "claude",
	}, "global", "")
	if err != nil {
		t.Fatalf("AddMemberToUnaffiliatedTeam() error = %v", err)
	}

	// Try to start the system team.
	_, err = s.StartTeam(StartTeamRequest{
		TeamID:     UnaffiliatedTeamID,
		LaunchMode: LaunchModeActiveSession,
	})
	if err == nil {
		t.Fatal("expected error when starting system team")
	}
	if !strings.Contains(err.Error(), "system team") {
		t.Fatalf("expected system team error, got: %v", err)
	}
}

func TestEnsureUnaffiliatedTeam_EmptyStorageLocationDefaultsToGlobal(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	team, err := s.EnsureUnaffiliatedTeam("", "")
	if err != nil {
		t.Fatalf("EnsureUnaffiliatedTeam() error = %v", err)
	}
	if team.StorageLocation != "global" {
		t.Fatalf("team.StorageLocation = %q, want %q", team.StorageLocation, "global")
	}
}

func TestDeleteTeam_RejectsSystemTeam(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	// Create the unaffiliated team first.
	_, err := s.EnsureUnaffiliatedTeam("global", "")
	if err != nil {
		t.Fatalf("EnsureUnaffiliatedTeam() error = %v", err)
	}

	// Try to delete the system team.
	err = s.DeleteTeam(UnaffiliatedTeamID, "global", "")
	if err == nil {
		t.Fatal("expected error when deleting system team")
	}
	if !strings.Contains(err.Error(), "cannot delete system team") {
		t.Fatalf("expected system team error, got: %v", err)
	}
}

func TestSaveTeam_RejectsSystemTeam(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	err := s.SaveTeam(TeamDefinition{
		ID:   UnaffiliatedTeamID,
		Name: "Hijacked",
	}, "")
	if err == nil {
		t.Fatal("expected error when saving system team")
	}
	if !strings.Contains(err.Error(), "cannot overwrite system team") {
		t.Fatalf("expected system team error, got: %v", err)
	}
}

func TestAddMemberToUnaffiliatedTeam_RejectsDuplicatePaneTitle(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	member := TeamMember{PaneTitle: "Lead", Role: "Lead", Command: "claude"}
	if err := s.AddMemberToUnaffiliatedTeam(member, "global", ""); err != nil {
		t.Fatalf("first add error = %v", err)
	}

	// Adding a second member with the same pane title should fail.
	err := s.AddMemberToUnaffiliatedTeam(member, "global", "")
	if err == nil {
		t.Fatal("expected error for duplicate pane title")
	}
	if !strings.Contains(err.Error(), "duplicate pane title") {
		t.Fatalf("expected duplicate pane title error, got: %v", err)
	}
}

func TestSaveUnaffiliatedTeamMembers(t *testing.T) {
	makeMember := func(title, role, cmd string) TeamMember {
		return TeamMember{PaneTitle: title, Role: role, Command: cmd}
	}

	tests := []struct {
		name       string
		setup      func(t *testing.T, s *Service)
		members    []TeamMember
		wantErr    string
		wantCount  int
		wantTitles []string
	}{
		{
			name:       "save 3 members from scratch",
			members:    []TeamMember{makeMember("A", "Dev", "cmd"), makeMember("B", "QA", "cmd"), makeMember("C", "Ops", "cmd")},
			wantCount:  3,
			wantTitles: []string{"A", "B", "C"},
		},
		{
			name: "overwrite existing members",
			setup: func(t *testing.T, s *Service) {
				t.Helper()
				for _, title := range []string{"Old1", "Old2", "Old3"} {
					if err := s.AddMemberToUnaffiliatedTeam(makeMember(title, "R", "cmd"), "global", ""); err != nil {
						t.Fatalf("setup: %v", err)
					}
				}
			},
			members:    []TeamMember{makeMember("New1", "Dev", "cmd"), makeMember("New2", "QA", "cmd")},
			wantCount:  2,
			wantTitles: []string{"New1", "New2"},
		},
		{
			name: "empty members removes all",
			setup: func(t *testing.T, s *Service) {
				t.Helper()
				if err := s.AddMemberToUnaffiliatedTeam(makeMember("X", "R", "cmd"), "global", ""); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			members:   []TeamMember{},
			wantCount: 0,
		},
		{
			name:    "duplicate pane title in input",
			members: []TeamMember{makeMember("Dup", "R", "cmd"), makeMember("Dup", "R2", "cmd2")},
			wantErr: "duplicate pane title",
		},
		{
			name:    "invalid member (empty fields)",
			members: []TeamMember{{}},
			wantErr: "validation failed",
		},
		{
			name:       "auto-creates team definition",
			members:    []TeamMember{makeMember("Solo", "Dev", "cmd")},
			wantCount:  1,
			wantTitles: []string{"Solo"},
		},
		{
			name: "preserves other team members",
			setup: func(t *testing.T, s *Service) {
				t.Helper()
				// Create a regular team with a member.
				err := s.SaveTeam(TeamDefinition{
					Name:             "RegularTeam",
					BootstrapDelayMs: 5000,
					Members:          []TeamMember{makeMember("RegMember", "Dev", "cmd")},
				}, "")
				if err != nil {
					t.Fatalf("setup SaveTeam: %v", err)
				}
			},
			members:    []TeamMember{makeMember("UnaffMember", "Dev", "cmd")},
			wantCount:  1,
			wantTitles: []string{"UnaffMember"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			s := NewService(testDeps(t, configPath))

			if tt.setup != nil {
				tt.setup(t, s)
			}

			err := s.SaveUnaffiliatedTeamMembers(tt.members, "")
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify by loading the unaffiliated team.
			team, err := s.EnsureUnaffiliatedTeam("global", "")
			if err != nil {
				t.Fatalf("EnsureUnaffiliatedTeam() error = %v", err)
			}
			if len(team.Members) != tt.wantCount {
				t.Fatalf("len(team.Members) = %d, want %d", len(team.Members), tt.wantCount)
			}
			for i, m := range team.Members {
				if m.Order != i {
					t.Errorf("member[%d].Order = %d, want %d", i, m.Order, i)
				}
				if m.TeamID != UnaffiliatedTeamID {
					t.Errorf("member[%d].TeamID = %q, want %q", i, m.TeamID, UnaffiliatedTeamID)
				}
			}
			for i, title := range tt.wantTitles {
				if i >= len(team.Members) {
					break
				}
				if team.Members[i].PaneTitle != title {
					t.Errorf("member[%d].PaneTitle = %q, want %q", i, team.Members[i].PaneTitle, title)
				}
			}
		})
	}
}

func TestSaveUnaffiliatedTeamMembers_PreservesOtherTeamMembers(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	// Create a regular team.
	err := s.SaveTeam(TeamDefinition{
		Name:             "MyTeam",
		BootstrapDelayMs: 5000,
		Members: []TeamMember{
			{PaneTitle: "T1", Role: "Dev", Command: "cmd"},
			{PaneTitle: "T2", Role: "QA", Command: "cmd"},
		},
	}, "")
	if err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	// Save unaffiliated members.
	err = s.SaveUnaffiliatedTeamMembers([]TeamMember{
		{PaneTitle: "U1", Role: "Dev", Command: "cmd"},
	}, "")
	if err != nil {
		t.Fatalf("SaveUnaffiliatedTeamMembers() error = %v", err)
	}

	// Verify regular team is untouched.
	teams, err := s.LoadTeams("")
	if err != nil {
		t.Fatalf("LoadTeams() error = %v", err)
	}
	var regularTeam *TeamDefinition
	for i := range teams {
		if teams[i].Name == "MyTeam" {
			regularTeam = &teams[i]
			break
		}
	}
	if regularTeam == nil {
		t.Fatal("regular team not found after SaveUnaffiliatedTeamMembers")
	}
	if len(regularTeam.Members) != 2 {
		t.Fatalf("regular team members = %d, want 2", len(regularTeam.Members))
	}
}

func TestSaveUnaffiliatedTeamMembers_DoesNotMutateInput(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	members := []TeamMember{
		{
			PaneTitle: "  Lead  ",
			Role:      "  Tester  ",
			Command:   "  claude  ",
			Args:      []string{"  --verbose  ", "   "},
		},
	}
	original := []TeamMember{
		{
			PaneTitle: members[0].PaneTitle,
			Role:      members[0].Role,
			Command:   members[0].Command,
			Args:      append([]string(nil), members[0].Args...),
		},
	}

	if err := s.SaveUnaffiliatedTeamMembers(members, ""); err != nil {
		t.Fatalf("SaveUnaffiliatedTeamMembers() error = %v", err)
	}

	if len(members) != len(original) {
		t.Fatalf("members length = %d, want %d", len(members), len(original))
	}
	if !reflect.DeepEqual(members, original) {
		t.Fatalf("input members mutated: got %#v want %#v", members[0], original[0])
	}
}

// startTestDeps is defined in service_test.go and shared across test files.
