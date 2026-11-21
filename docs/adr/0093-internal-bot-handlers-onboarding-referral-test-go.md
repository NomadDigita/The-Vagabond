# ADR 0093: Decisions from `internal/bot/handlers/onboarding_referral_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/onboarding_referral_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestGenerateReferralCode_Deterministic verifies that the same Telegram ID

Removed from `internal/bot/handlers/onboarding_referral_test.go:5`.

~~~~text
TestGenerateReferralCode_Deterministic verifies that the same Telegram ID
always produces the same code (needed so a code set at signup matches a
code independently regenerated later, e.g. by /refer's fallback path).
~~~~


## TestGenerateReferralCode_Unique verifies distinct Telegram IDs never

Removed from `internal/bot/handlers/onboarding_referral_test.go:19`.

~~~~text
TestGenerateReferralCode_Unique verifies distinct Telegram IDs never
collide on the same code — the bug being fixed here is the old
`telegramID % 1_000_000` scheme, which could map two different players to
an identical code.
~~~~


## differ by exactly 1_000_000,000 not 1_000_000 but still worth checking

Removed from `internal/bot/handlers/onboarding_referral_test.go:26`.

~~~~text
differ by exactly 1_000_000,000 not 1_000_000 but still worth checking
~~~~


## TestGenerateReferralCode_NoOldCollisionPattern specifically checks two IDs

Removed from `internal/bot/handlers/onboarding_referral_test.go:39`.

~~~~text
TestGenerateReferralCode_NoOldCollisionPattern specifically checks two IDs
that WOULD have collided under the old `telegramID % 1_000_000` scheme no
longer produce the same code.
~~~~


## TestReferralMilestones_Ascending ensures milestone tiers are defined in

Removed from `internal/bot/handlers/onboarding_referral_test.go:63`.

~~~~text
TestReferralMilestones_Ascending ensures milestone tiers are defined in
strictly increasing order of Count, which the claiming loop in
HandleFactionCallback relies on for correct "first unclaimed tier" logic.
~~~~
