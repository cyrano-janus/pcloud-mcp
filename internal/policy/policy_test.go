package policy

import "testing"

func TestAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, op           string
		id                 int64
		write, owner, want bool
	}{
		{"read", "list_folder", 7, false, true, true},
		{"wrong identity", "list_folder", 8, true, true, false},
		{"missing identity", "whoami", 0, false, true, false},
		{"foreign object", "get_file_info", 7, true, false, false},
		{"read only", "create_folder", 7, false, true, false},
		{"allowed write", "copy_file", 7, true, true, true},
		{"delete denied", "delete_file", 7, true, true, false},
		{"overwrite denied", "overwrite_file", 7, true, true, false},
		{"unsafe rename denied", "rename_file", 7, true, true, false},
		{"unsafe move denied", "move_file", 7, true, true, false},
		{"unknown denied", "anything", 7, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Policy{UserID: 7, EnableWrites: true}).Authorize(Principal{UserID: tc.id, Write: tc.write}, tc.op, tc.owner)
			if (err == nil) != tc.want {
				t.Fatalf("allowed=%v want=%v", err == nil, tc.want)
			}
		})
	}
	if (Policy{UserID: 7}).Authorize(Principal{UserID: 7, Write: true}, "copy_file", true) == nil {
		t.Fatal("server write switch bypassed")
	}
}

func TestName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../x", "a/b", "a\\b", "a\x00b", "a\nb"} {
		if ValidName(name) {
			t.Fatalf("accepted %q", name)
		}
	}
	if !ValidName("Urlaub 2026 – Bild.png") {
		t.Fatal("valid Unicode filename rejected")
	}
}
func FuzzName(f *testing.F) {
	for _, v := range []string{"ok.txt", "../escape", "a\\b", "\x00"} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v string) {
		if ValidName(v) && (len(v) == 0 || len(v) > 255) {
			t.Fatal("invalid length accepted")
		}
	})
}
