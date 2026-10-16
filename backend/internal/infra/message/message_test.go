package message

import "testing"

func TestMessageAttributes(t *testing.T) {
	t.Run("scope omitted when unset", func(t *testing.T) {
		got := messageAttributes("", "")
		if len(got) != 1 || *got["source"].StringValue != "online" {
			t.Fatalf("want only source=online, got %v", got)
		}
	})
	t.Run("scope stamped when set", func(t *testing.T) {
		got := messageAttributes("t-uuid", "l-uuid")
		if *got["tenant_id"].StringValue != "t-uuid" || *got["location_id"].StringValue != "l-uuid" {
			t.Fatalf("scope attrs missing: %v", got)
		}
	})
}
