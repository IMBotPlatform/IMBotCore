package command

import (
	"reflect"
	"testing"
)

func TestParserParseCommandWithTrailingWordJoiner(t *testing.T) {
	parsed := NewParser().Parse("/devops\u2060 192.168.1.63 上部署了哪些 docker 服务")

	if !parsed.IsCommand {
		t.Fatal("expected command")
	}
	if got, want := parsed.Tokens, []string{"devops", "192.168.1.63", "上部署了哪些", "docker", "服务"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
	if got, want := parsed.ArgumentRaw, "192.168.1.63 上部署了哪些 docker 服务"; got != want {
		t.Fatalf("ArgumentRaw = %q, want %q", got, want)
	}
}

func TestParserParseMentionCommandWithWordJoiner(t *testing.T) {
	parsed := NewParser().Parse("/devops\u2060@QBot 192.168.1.63")

	if !parsed.IsCommand {
		t.Fatal("expected command")
	}
	if got, want := parsed.Tokens, []string{"devops", "192.168.1.63"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
}
