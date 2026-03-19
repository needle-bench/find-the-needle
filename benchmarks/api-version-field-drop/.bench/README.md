# api-version-field-drop

## Difficulty
Medium

## Source
Community-submitted

## Environment
Go 1.21, Alpine Linux

## The bug
The v2 API user struct in `app/models.go` is missing three fields that exist in v1 (`avatar_url`, `bio`, `location`). The `ToV2()` conversion function faithfully maps only the fields defined in the v2 struct, so these fields silently disappear from v2 responses. The v2 API was supposed to be a strict superset of v1.

## Why Medium
Requires comparing two API versions, understanding struct-to-struct conversion, and identifying that the data model (not the handler logic) is the problem. The agent must diff the v1 and v2 structs and the corresponding conversion functions to find the missing fields.

## Expected fix
Add the missing fields (`AvatarURL`, `Bio`, `Location`) to the `UserV2` struct with their JSON tags, and map them in the `ToV2()` conversion function.

## Pinned at
Anonymized snapshot, original repo not disclosed
