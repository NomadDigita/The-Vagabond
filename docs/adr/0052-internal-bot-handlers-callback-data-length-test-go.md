# ADR 0052: Decisions from `internal/bot/handlers/callback_data_length_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/callback_data_length_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestRoadEncounterCallbackData_FitsTelegramLimit guards against the bug

Removed from `internal/bot/handlers/callback_data_length_test.go:10`.

~~~~text
TestRoadEncounterCallbackData_FitsTelegramLimit guards against the bug
reported as "the button to go to that place won't respond" for raids
passing another raid or a base mid-journey: Telegram inline buttons cap
callback_data at 64 bytes, and Telegram silently drops any button whose
data exceeds that (the message still sends, but the button does
nothing when tapped - no error, no response). road_encounter used to
carry both an encounter UUID and a raid UUID (~95 bytes with the "\f"
prefix and separators), and rbe (formerly the much longer
"road_base_encounter") was right at the edge for its "continue" action.
Both now carry only a single UUID.

clan_app_accept/clan_app_reject (now cl_acc/cl_rej) is included too:
it combines a Telegram user ID with a clan UUID, and was found during
the same audit sitting at 63-64 bytes with a 10-digit user ID - not
broken yet, but one digit of ID growth away from silently breaking the
same way. Shortened its prefix to leave real margin rather than
leaving it fragile.
~~~~


## 36 bytes, representative of a real Postgres UUID

Removed from `internal/bot/handlers/callback_data_length_test.go:29`.

~~~~text
36 bytes, representative of a real Postgres UUID
~~~~


## Guard the specific regression: road_encounter must never carry a

Removed from `internal/bot/handlers/callback_data_length_test.go:56`.

~~~~text
Guard the specific regression: road_encounter must never carry a
second UUID (the raid ID) again, since that's what pushed it over
the limit in the first place. The caller's raid ID is now derived
server-side in HandleRoadEncounterCallback instead.
~~~~


## TestClanKickPromoteCallbackData_FitsTelegramLimit covers clan_kick and

Removed from `internal/bot/handlers/callback_data_length_test.go:66`.

~~~~text
TestClanKickPromoteCallbackData_FitsTelegramLimit covers clan_kick and
clan_promote, added to the styled (colored) button rollout - a single
Telegram user ID, well within the limit, but included so this test
file stays the single place documenting every button family's
callback_data budget rather than leaving future additions unverified.
~~~~


## 12 digits, margin over a typical current Telegram user ID

Removed from `internal/bot/handlers/callback_data_length_test.go:73`.

~~~~text
12 digits, margin over a typical current Telegram user ID
~~~~


## (formerly clan_app_accept/clan_app_reject), which packs a Telegram

Removed from `internal/bot/handlers/callback_data_length_test.go:86`.

~~~~text
(formerly clan_app_accept/clan_app_reject), which packs a Telegram
user ID and a clan UUID into callback_data. Telegram user IDs are
currently up to 10 digits but have grown over the platform's history
and aren't guaranteed to stay that length, so this asserts real
margin under the 64-byte cap rather than just scraping under it -
a regression here should be visible well before it actually breaks.
~~~~


## 12 digits: two more than a typical current Telegram user ID, to

Removed from `internal/bot/handlers/callback_data_length_test.go:95`.

~~~~text
12 digits: two more than a typical current Telegram user ID, to
verify margin rather than just today's exact case.
~~~~


## leave at least 4 bytes of headroom under Telegram's 64-byte cap

Removed from `internal/bot/handlers/callback_data_length_test.go:99`.

~~~~text
leave at least 4 bytes of headroom under Telegram's 64-byte cap
~~~~
