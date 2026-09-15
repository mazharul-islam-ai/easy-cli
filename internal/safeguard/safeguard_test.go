package safeguard

import "testing"

func TestCheckSudo(t *testing.T) {
	err := Check(func(k string) string {
		if k == "SUDO_USER" {
			return "alice"
		}
		return ""
	}, 1000)
	if err != ErrSudo {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRoot(t *testing.T) {
	err := Check(func(string) string { return "" }, 0)
	if err != ErrRoot {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRootAllowed(t *testing.T) {
	err := Check(func(k string) string {
		if k == "EASY_ALLOW_ROOT" {
			return "1"
		}
		return ""
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckNormalUser(t *testing.T) {
	if err := Check(func(string) string { return "" }, 1000); err != nil {
		t.Fatal(err)
	}
}
